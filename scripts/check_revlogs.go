package main

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"time"

	_ "github.com/lib/pq"
)

const (
	userID  = "f02fb452-a9ea-42fb-b5f2-4c5626369fd7"
	studyDB = "postgresql://neondb_owner:npg_u4lhpDfRgj6E@ep-flat-cloud-ao5gwidz-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
)

type Revlog struct {
	CardID      string
	Rating      int
	ReviewTime  time.Time
	RealDays    int
	DeltaT      int
	I           int
}

func main() {
	db, err := sql.Open("postgres", studyDB)
	if err != nil {
		log.Fatalf("failed to open study DB: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT card_id, rating, review_time FROM revlogs WHERE user_id = $1`, userID)
	if err != nil {
		log.Fatalf("failed to query: %v", err)
	}
	defer rows.Close()

	var logs []Revlog
	for rows.Next() {
		var l Revlog
		if err := rows.Scan(&l.CardID, &l.Rating, &l.ReviewTime); err != nil {
			log.Fatalf("failed to scan: %v", err)
		}
		logs = append(logs, l)
	}

	// 1. Sort by card_id and review_time ASC
	sort.Slice(logs, func(i, j int) bool {
		if logs[i].CardID != logs[j].CardID {
			return logs[i].CardID < logs[j].CardID
		}
		return logs[i].ReviewTime.Before(logs[j].ReviewTime)
	})

	// 2. Calculate real_days (UTC timezone, next_day_starts_at = 4)
	for idx := range logs {
		t := logs[idx].ReviewTime.UTC()
		adjusted := t.Add(-4 * time.Hour)
		// Convert to day index (Unix days)
		logs[idx].RealDays = int(adjusted.Unix() / 86400)
	}

	// 3. Calculate delta_t (diff of real_days) and i (groupby card_id cumcount + 1)
	cardCounts := make(map[string]int)
	for idx := range logs {
		cardID := logs[idx].CardID
		cardCounts[cardID]++
		logs[idx].I = cardCounts[cardID]

		if idx == 0 {
			logs[idx].DeltaT = 0
		} else {
			logs[idx].DeltaT = logs[idx].RealDays - logs[idx-1].RealDays
		}

		if logs[idx].I == 1 {
			logs[idx].DeltaT = -1
		}
	}

	// 4. Count and filter
	var total, eligible int
	for _, l := range logs {
		total++
		if l.I > 1 && l.DeltaT > 0 {
			eligible++
		}
	}

	fmt.Printf("Total: %d, Eligible (i > 1 && delta_t > 0): %d\n", total, eligible)

	// Print some samples of i > 1
	fmt.Println("\nSamples of i > 1 logs:")
	count := 0
	for _, l := range logs {
		if l.I > 1 {
			count++
			fmt.Printf("  Card: %s, i: %d, delta_t: %d, real_days: %d, review_time: %s\n",
				l.CardID, l.I, l.DeltaT, l.RealDays, l.ReviewTime)
			if count >= 20 {
				break
			}
		}
	}
}
