package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Item = struktur data sederhana yang disimpan backend.
type Item struct {
	ID        int       `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

// store = penyimpanan IN-MEMORY (hilang kalau Pod restart).
// Nanti di fase storage, ini diganti baca/tulis ke file di volume (PVC).
// Pakai mutex biar aman dari race condition saat banyak request barengan.
var (
	store  = []Item{}
	nextID = 1
	mu     sync.Mutex
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Seed data awal biar nggak kosong pas pertama kali diakses
	store = append(store,
		Item{ID: nextID, Text: "Halo dari backend Golang", CreatedAt: time.Now()},
	)
	nextID++

	mux := http.NewServeMux()

	// GET /api/items  -> daftar item
	// POST /api/items -> tambah item (body: {"text":"..."} )
	mux.HandleFunc("/api/items", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			mu.Lock()
			defer mu.Unlock()
			writeJSON(w, http.StatusOK, store)

		case http.MethodPost:
			var body struct {
				Text string `json:"text"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Text == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "field 'text' wajib diisi"})
				return
			}
			mu.Lock()
			item := Item{ID: nextID, Text: body.Text, CreatedAt: time.Now()}
			store = append(store, item)
			nextID++
			mu.Unlock()
			writeJSON(w, http.StatusCreated, item)

		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method tidak didukung"})
		}
	})

	// Health check buat probe Kubernetes (liveness & readiness)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Info buat tau Pod mana yang nanganin request (lihat load balancing antar Pod)
	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		host, _ := os.Hostname()
		writeJSON(w, http.StatusOK, map[string]string{
			"service":  "backend-golang",
			"hostname": host,
		})
	})

	log.Printf("backend listening on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
