package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/lib/pq"
)

const (
	studyDB = "postgresql://neondb_owner:npg_u4lhpDfRgj6E@ep-flat-cloud-ao5gwidz-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
)

func main() {
	db, err := sql.Open("postgres", studyDB)
	if err != nil {
		log.Fatalf("failed to open study DB: %v", err)
	}
	defer db.Close()

	// 1. Describe table columns or check existing records
	rows, err := db.Query(`SELECT user_id, version, is_active, weights FROM user_fsrs_weights LIMIT 5`)
	if err != nil {
		log.Fatalf("failed to query user_fsrs_weights: %v", err)
	}
	defer rows.Close()

	fmt.Println("Existing rows in user_fsrs_weights:")
	count := 0
	for rows.Next() {
		count++
		var userID, version string
		var isActive bool
		var weights pq.Float64Array
		if err := rows.Scan(&userID, &version, &isActive, &weights); err != nil {
			log.Fatalf("failed to scan: %v", err)
		}
		fmt.Printf("User: %s, Version: %s, IsActive: %t, Weights length: %d, Weights: %v\n",
			userID, version, isActive, len(weights), weights)
	}
	if count == 0 {
		fmt.Println("No records found in user_fsrs_weights table.")
	}
}
