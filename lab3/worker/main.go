package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	dsn := env("DATABASE_URL", "postgres://shop:shop@postgres:5432/shop?sslmode=disable")
	interval := 3 * time.Second

	var db *sql.DB
	for i := 0; i < 60; i++ {
		var err error
		db, err = sql.Open("postgres", dsn)
		if err == nil {
			err = db.Ping()
		}
		if err == nil {
			break
		}
		log.Printf("worker: waiting for postgres: %v", err)
		time.Sleep(2 * time.Second)
	}

	log.Println("worker started")
	for {
		rows, err := db.Query(`UPDATE orders SET processed = TRUE
			WHERE id IN (SELECT id FROM orders WHERE processed = FALSE ORDER BY id LIMIT 10)
			RETURNING id`)
		if err != nil {
			log.Printf("worker: update error: %v", err)
			time.Sleep(interval)
			continue
		}
		var ids []int
		for rows.Next() {
			var id int
			rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		if len(ids) > 0 {
			log.Printf("worker: processed %d orders: %v", len(ids), ids)
		}
		time.Sleep(interval)
	}
}
