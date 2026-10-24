package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dsn       string
	version   string
	failFlag  bool
	flakyFlag bool
	db        *sql.DB
	dbMu      sync.Mutex
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "http_requests_total", Help: "Total HTTP requests"},
		[]string{"method", "path", "status", "version"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "http_request_duration_seconds", Help: "HTTP request duration", Buckets: prometheus.DefBuckets},
		[]string{"method", "path", "version"},
	)
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	version = env("VERSION", "v1")
	failFlag = env("FAIL", "false") == "true"
	flakyFlag = env("FLAKY", "false") == "true"
	dsn = env("DATABASE_URL", "postgres://shop:shop@shop-shop-postgres-rw:5432/shop?sslmode=disable")

	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/version", versionHandler)
	mux.HandleFunc("/order", metricsMiddleware(orderHandler))
	mux.HandleFunc("/orders", metricsMiddleware(ordersHandler))

	log.Printf("Starting %s (FAIL=%v, FLAKY=%v)", version, failFlag, flakyFlag)
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func metricsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rec.status), version).Inc()
		httpRequestDuration.WithLabelValues(r.Method, r.URL.Path, version).Observe(time.Since(start).Seconds())
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"version": version})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	// Имитация сбоя для v2 (плохой релиз)
	if failFlag {
		http.Error(w, "simulated failure (FAIL=true)", http.StatusInternalServerError)
		return
	}

	// Имитация "флаки" ошибки для v1 (для части 4 - надежность)
	if flakyFlag && rand.Intn(100) < 50 { // 50% запросов падают
		http.Error(w, "flaky error (FLAKY=true)", http.StatusInternalServerError)
		return
	}

	var body struct {
		Item string `json:"item"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Item == "" {
		http.Error(w, "item required", http.StatusBadRequest)
		return
	}

	d, err := getDB()
	if err != nil {
		http.Error(w, "db unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	var id int
	if err := d.QueryRow(`INSERT INTO orders (item) VALUES ($1) RETURNING id`, body.Item).Scan(&id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"id": id, "item": body.Item})
}

func ordersHandler(w http.ResponseWriter, r *http.Request) {
	d, err := getDB()
	if err != nil {
		http.Error(w, "db unavailable: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	rows, err := d.Query(`SELECT id, item, processed, created_at FROM orders ORDER BY id DESC LIMIT 100`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Order struct {
		ID        int       `json:"id"`
		Item      string    `json:"item"`
		Processed bool      `json:"processed"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := []Order{}
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.Item, &o.Processed, &o.CreatedAt); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out = append(out, o)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func getDB() (*sql.DB, error) {
	dbMu.Lock()
	defer dbMu.Unlock()
	if db != nil {
		if err := db.Ping(); err == nil {
			return db, nil
		}
		db.Close()
		db = nil
	}
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	d.SetConnMaxLifetime(time.Minute)
	if err := d.Ping(); err != nil {
		d.Close()
		return nil, err
	}
	_, err = d.Exec(`CREATE TABLE IF NOT EXISTS orders (id SERIAL PRIMARY KEY, item TEXT NOT NULL, processed BOOLEAN NOT NULL DEFAULT FALSE, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`)
	if err != nil {
		d.Close()
		return nil, err
	}
	db = d
	return db, nil
}
