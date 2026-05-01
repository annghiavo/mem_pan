package parser

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

// ParseCSV parses tab- or comma-separated flashcard files.
// Each row must have at least two columns: front and back.
// Rows with blank front or back are skipped.
func ParseCSV(data []byte) ([]ParsedCard, error) {
	data = stripBOM(data)
	sep := detectSeparator(data)

	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = sep
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	r.LazyQuotes = true

	var cards []ParsedCard
	line := 0
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv line %d: %w", line+1, err)
		}
		line++

		if len(record) < 2 {
			continue
		}
		front := strings.TrimSpace(record[0])
		back := strings.TrimSpace(record[1])
		if front == "" || back == "" {
			continue
		}
		cards = append(cards, ParsedCard{Front: front, Back: back})
	}
	return cards, nil
}

// detectSeparator picks '\t' when tabs outnumber commas, else ','.
func detectSeparator(data []byte) rune {
	tabs := bytes.Count(data, []byte{'\t'})
	commas := bytes.Count(data, []byte{','})
	if tabs > commas {
		return '\t'
	}
	return ','
}

// stripBOM removes a UTF-8 byte-order mark if present.
func stripBOM(data []byte) []byte {
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return data[3:]
	}
	return data
}
