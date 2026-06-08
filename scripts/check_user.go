package main

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
)

const (
	userID = "c12adea4-2dc6-4303-be27-4ab1896bb8c8"

	authDB = "postgresql://neondb_owner:npg_Y7xvdHo9qZAC@ep-spring-union-aos8717l-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
	notifDB = "postgresql://neondb_owner:npg_06EigybVJlWo@ep-bold-dawn-aosg89qs-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
	statsDB = "postgresql://neondb_owner:npg_vwY1GqBQMim2@ep-quiet-lab-aoxoxnpu-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
	studyDB = "postgresql://neondb_owner:npg_u4lhpDfRgj6E@ep-flat-cloud-ao5gwidz-pooler.c-2.ap-southeast-1.aws.neon.tech/neondb?sslmode=require"
)

func main() {
	log.Println("--- Checking Auth DB ---")
	checkAuth()

	log.Println("\n--- Checking Stats DB ---")
	checkStats()

	log.Println("\n--- Checking Notification DB ---")
	checkNotification()

	log.Println("\n--- Checking Study DB ---")
	checkStudy()
}

func checkStats() {
	db, err := sql.Open("postgres", statsDB)
	if err != nil {
		log.Fatalf("failed to open stats DB: %v", err)
	}
	defer db.Close()

	var uID, reminderTime string
	var optimalWeekday, optimalWeekend sql.NullInt64
	var streak int
	var lastStudied sql.NullString

	query := `SELECT user_id, reminder_local_time, optimal_hour_weekday, optimal_hour_weekend, current_streak, last_studied_date FROM user_stats WHERE user_id = $1`
	err = db.QueryRow(query, userID).Scan(&uID, &reminderTime, &optimalWeekday, &optimalWeekend, &streak, &lastStudied)
	if err == sql.ErrNoRows {
		log.Printf("ERROR: User %s does NOT exist in user_stats table!", userID)
		return
	} else if err != nil {
		log.Fatalf("failed to query user_stats: %v", err)
	}

	fmt.Printf("User ID: %s\n", uID)
	fmt.Printf("Reminder Local Time: %s\n", reminderTime)
	fmt.Printf("Optimal Hour Weekday: %+v\n", optimalWeekday)
	fmt.Printf("Optimal Hour Weekend: %+v\n", optimalWeekend)
	fmt.Printf("Current Streak: %d\n", streak)
	fmt.Printf("Last Studied Date: %+v\n", lastStudied)
}

func checkNotification() {
	db, err := sql.Open("postgres", notifDB)
	if err != nil {
		log.Fatalf("failed to open notif DB: %v", err)
	}
	defer db.Close()

	// Check FCM tokens
	rows, err := db.Query(`SELECT token, device_name, updated_at FROM fcm_tokens WHERE user_id = $1`, userID)
	if err != nil {
		log.Fatalf("failed to query fcm_tokens: %v", err)
	}
	defer rows.Close()

	tokenCount := 0
	for rows.Next() {
		tokenCount++
		var token, deviceName string
		var updatedAt time.Time
		if err := rows.Scan(&token, &deviceName, &updatedAt); err != nil {
			log.Fatalf("failed to scan token: %v", err)
		}
		tokenTrunc := token
		if len(token) > 20 {
			tokenTrunc = token[:17] + "..."
		}
		fmt.Printf("Token #%d: %s (%q), updated_at: %s\n", tokenCount, tokenTrunc, deviceName, updatedAt)
	}
	if tokenCount == 0 {
		fmt.Println("WARNING: 0 FCM tokens found for this user in fcm_tokens table!")
	}

	// Check recent logs
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM notification_log WHERE user_id = $1 AND notification_type = 'study_reminder' AND created_at >= date_trunc('day', now())`, userID).Scan(&count)
	if err != nil {
		log.Fatalf("failed to query notification_log: %v", err)
	}
	fmt.Printf("Notification Log count for today: %d\n", count)
}

func checkStudy() {
	db, err := sql.Open("postgres", studyDB)
	if err != nil {
		log.Fatalf("failed to open study DB: %v", err)
	}
	defer db.Close()

	// Count total cards and due cards
	var totalCards, nonNewCards, dueCards int
	err = db.QueryRow(`SELECT COUNT(*) FROM user_cards WHERE user_id = $1`, userID).Scan(&totalCards)
	if err != nil {
		log.Fatalf("failed to query total cards: %v", err)
	}
	err = db.QueryRow(`SELECT COUNT(*) FROM user_cards WHERE user_id = $1 AND state != 'new'`, userID).Scan(&nonNewCards)
	if err != nil {
		log.Fatalf("failed to query non-new cards: %v", err)
	}
	// We check due cards (next_review_date <= now)
	err = db.QueryRow(`SELECT COUNT(*) FROM user_cards WHERE user_id = $1 AND state != 'new' AND next_review_date <= NOW()`, userID).Scan(&dueCards)
	if err != nil {
		log.Fatalf("failed to query due cards: %v", err)
	}

	fmt.Printf("Total Cards in user_cards: %d\n", totalCards)
	fmt.Printf("Non-New Cards (learned): %d\n", nonNewCards)
	fmt.Printf("Due Cards right now (next_review_date <= NOW): %d\n", dueCards)

	// If total cards > 0, let's print a sample next_review_date
	if totalCards > 0 {
		rows, err := db.Query(`SELECT card_id, state, next_review_date FROM user_cards WHERE user_id = $1 LIMIT 5`, userID)
		if err == nil {
			defer rows.Close()
			fmt.Println("Sample User Cards:")
			for rows.Next() {
				var cardID, state string
				var nextReview time.Time
				if err := rows.Scan(&cardID, &state, &nextReview); err == nil {
					fmt.Printf("  Card %s: state=%s, next_review_date=%s\n", cardID, state, nextReview)
				}
			}
		}
	}
}

func checkAuth() {
	db, err := sql.Open("postgres", authDB)
	if err != nil {
		log.Fatalf("failed to open auth DB: %v", err)
	}
	defer db.Close()

	var uID, username, email, timezone string
	query := `SELECT user_id, username, email, timezone FROM users WHERE user_id = $1`
	err = db.QueryRow(query, userID).Scan(&uID, &username, &email, &timezone)
	if err == sql.ErrNoRows {
		log.Printf("ERROR: User %s does NOT exist in auth users table!", userID)
		return
	} else if err != nil {
		log.Fatalf("failed to query auth users: %v", err)
	}

	fmt.Printf("User ID: %s\n", uID)
	fmt.Printf("Username: %s\n", username)
	fmt.Printf("Email: %s\n", email)
	fmt.Printf("Timezone: %s\n", timezone)
}

