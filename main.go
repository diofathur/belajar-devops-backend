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

var (
	store    = []Item{}
	nextID   = 1
	mu       sync.Mutex
	dataFile = os.Getenv("DATA_FILE")
)

// load = baca data dari file ke memori pas startup
func load() {
	if dataFile == "" {
		dataFile = "items.json"
	}
	b, err := os.ReadFile(dataFile)
	if err != nil {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	_ = json.Unmarshal(b, &store)
	for _, it := range store {
		if it.ID >= nextID {
			nextID = it.ID + 1
		}
	}
}

// save = tulis store ke file (dipanggil tiap ada perubahan)
func save() {
	b, _ := json.MarshalIndent(store, "", "  ")
	_ = os.WriteFile(dataFile, b, 0644)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Baca data dari volume (kalau ada file-nya)
	load()

	// Seed cuma kalau store masih kosong (file belum pernah dibuat)
	if len(store) == 0 {
		store = append(store,
			Item{ID: nextID, Text: "Halo dari backend Golang", CreatedAt: time.Now()},
		)
		nextID++
		save()
	}

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
			save()
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

	// Info buat tau Pod mana yang nanganin request
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
