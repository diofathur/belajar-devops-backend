package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq" // driver PostgreSQL
)

type Item struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

var db *sql.DB

func main() {
	port := getenv("PORT", "8080")
	host := getenv("DB_HOST", "localhost")
	dbPort := getenv("DB_PORT", "5432")
	name := getenv("DB_NAME", "belajardb")
	user := getenv("DB_USER", "appuser")
	pass := os.Getenv("DB_PASSWORD") // dari K8s Secret

	dsn := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=require",
		host, dbPort, name, user, pass)

	var err error
	db, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("gagal open db: %v", err)
	}
	defer db.Close()

	if err := waitForDB(10); err != nil {
		log.Fatalf("gagal konek db: %v", err)
	}
	if err := initSchema(); err != nil {
		log.Fatalf("gagal init schema: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := listItems()
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 200, items)
		case http.MethodPost:
			var body struct {
				Text string `json:"text"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
				writeJSON(w, 400, map[string]string{"error": "field 'text' wajib diisi"})
				return
			}
			item, err := insertItem(body.Text)
			if err != nil {
				writeJSON(w, 500, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, 201, item)
		default:
			writeJSON(w, 405, map[string]string{"error": "method tidak didukung"})
		}
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(); err != nil {
			writeJSON(w, 503, map[string]string{"status": "db down"})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		h, _ := os.Hostname()
		writeJSON(w, 200, map[string]string{"service": "backend-golang", "hostname": h})
	})

	log.Printf("backend listening on %s (db=%s)", port, host)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func waitForDB(tries int) error {
	var err error
	for i := 0; i < tries; i++ {
		if err = db.Ping(); err == nil {
			return nil
		}
		log.Printf("db belum siap (%d/%d): %v", i+1, tries, err)
		time.Sleep(3 * time.Second)
	}
	return err
}

func initSchema() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (
		id SERIAL PRIMARY KEY,
		text TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT now())`)
	if err != nil {
		return err
	}
	var n int
	db.QueryRow("SELECT COUNT(*) FROM items").Scan(&n)
	if n == 0 {
		_, err = db.Exec("INSERT INTO items (text) VALUES ($1)", "Halo dari backend + RDS")
	}
	return err
}

func listItems() ([]Item, error) {
	rows, err := db.Query("SELECT id, text, created_at FROM items ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Text, &it.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func insertItem(text string) (Item, error) {
	var it Item
	err := db.QueryRow(
		"INSERT INTO items (text) VALUES ($1) RETURNING id, text, created_at",
		text).Scan(&it.ID, &it.Text, &it.CreatedAt)
	return it, err
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
