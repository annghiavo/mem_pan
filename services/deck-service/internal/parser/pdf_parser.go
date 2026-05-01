package parser

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/ledongthuc/pdf"
)

// lineStartRe matches a leading row-number token such as "1", "1.", "42."
var lineStartRe = regexp.MustCompile(`^\d+\.?$`)

// numberRe matches a bare integer line (plain-text fallback).
var numberRe = regexp.MustCompile(`^\d+$`)

// ParsePDF extracts flashcard pairs from a two-column PDF layout.
//
// Primary strategy: coordinate-based column splitting via GetTextByRow().
// Fallback strategy: plain-text line parsing when coordinates are all zero
// (e.g. Quizlet-exported PDFs whose content stream lacks position data).
func ParsePDF(data []byte) ([]ParsedCard, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}

	cards, coordinatesBroken := extractByCoordinates(reader)
	if coordinatesBroken || len(cards) == 0 {
		return extractByPlainText(reader)
	}
	return cards, nil
}

// extractByCoordinates attempts the coordinate-based column-split approach.
// Returns (cards, true) if coordinates appear broken (all X == 0).
func extractByCoordinates(reader *pdf.Reader) ([]ParsedCard, bool) {
	var cards []ParsedCard
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			return nil, true
		}
		if coordinatesAllZero(rows) {
			return nil, true
		}
		for _, row := range rows {
			if card, ok := rowToCard(row.Content); ok {
				cards = append(cards, card)
			}
		}
	}
	return cards, false
}

func coordinatesAllZero(rows pdf.Rows) bool {
	for _, row := range rows {
		for _, t := range row.Content {
			if t.X != 0 {
				return false
			}
		}
	}
	return true
}

// extractByPlainText parses the structured plain-text output produced by
// Quizlet-style PDFs. Each card appears as four consecutive non-empty lines:
//
//	<number>
//	.
//	<term>
//	<definition>
func extractByPlainText(reader *pdf.Reader) ([]ParsedCard, error) {
	var lines []string
	for pageNum := 1; pageNum <= reader.NumPage(); pageNum++ {
		page := reader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		for _, l := range strings.Split(text, "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				lines = append(lines, l)
			}
		}
	}

	var cards []ParsedCard
	for i := 0; i+3 < len(lines); {
		if numberRe.MatchString(lines[i]) && lines[i+1] == "." {
			front := strings.TrimSpace(lines[i+2])
			back := strings.TrimSpace(lines[i+3])
			if front != "" && back != "" {
				cards = append(cards, ParsedCard{Front: front, Back: back})
			}
			i += 4
		} else {
			i++
		}
	}

	return cards, nil
}

// rowToCard converts one visual row (left-to-right sorted text fragments)
// into a ParsedCard using the maximum-gap heuristic.
func rowToCard(row pdf.TextHorizontal) (ParsedCard, bool) {
	if len(row) < 2 {
		return ParsedCard{}, false
	}

	first := strings.TrimSpace(row[0].S)
	if !lineStartRe.MatchString(first) {
		return ParsedCard{}, false
	}

	rest := []pdf.Text(row[1:])
	if len(rest) < 2 {
		return ParsedCard{}, false
	}

	maxGap := -1.0
	splitAt := 0
	for i := 0; i < len(rest)-1; i++ {
		gap := rest[i+1].X - rest[i].X
		if gap > maxGap {
			maxGap = gap
			splitAt = i
		}
	}

	var frontParts, backParts []string
	for i, t := range rest {
		s := strings.TrimSpace(t.S)
		if s == "" {
			continue
		}
		if i <= splitAt {
			frontParts = append(frontParts, s)
		} else {
			backParts = append(backParts, s)
		}
	}

	front := strings.Join(frontParts, " ")
	back := strings.Join(backParts, " ")
	if front == "" || back == "" {
		return ParsedCard{}, false
	}
	return ParsedCard{Front: front, Back: back}, true
}
