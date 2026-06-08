package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

const (
	userID  = "c12adea4-2dc6-4303-be27-4ab1896bb8c8"
	studyDB = "postgresql://neondb_owner:npg_u4lhpDfRgj6E@ep-flat-cloud-ao5gwidz-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
)

type Log struct {
	CardID     string    `json:"card_id"`
	Rating     int       `json:"rating"`
	ReviewTime time.Time `json:"review_time"`
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

	var logs []Log
	for rows.Next() {
		var l Log
		if err := rows.Scan(&l.CardID, &l.Rating, &l.ReviewTime); err != nil {
			log.Fatalf("failed to scan: %v", err)
		}
		logs = append(logs, l)
	}

	f, err := os.Create("scripts/revlogs.json")
	if err != nil {
		log.Fatalf("failed to create file: %v", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(logs); err != nil {
		log.Fatalf("failed to encode: %v", err)
	}
	log.Println("Exported revlogs to scripts/revlogs.json")
}
