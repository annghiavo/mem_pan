package main

import (
	"database/sql"
	"log"
	"os"

	"mem_pan/api"

	_ "github.com/lib/pq"
)

func main() {
	dbURL := firstNonEmpty(
		os.Getenv("DB_URL"),
		os.Getenv("DIRECT_URL"),
		os.Getenv("DATABASE_URL"),
	)
	if dbURL == "" {
		log.Fatal("DB_URL, DIRECT_URL, or DATABASE_URL is required")
	}

	serverAddress := os.Getenv("SERVER_ADDRESS")
	if serverAddress == "" {
		serverAddress = ":8080"
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	server := api.NewServer(db)
	if err := server.Start(serverAddress); err != nil {
		log.Fatal(err)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
