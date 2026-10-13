package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	dsn        string
	healthFail bool
	db         *sql.DB
	dbMu       sync.Mutex
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests by method, path, status",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	healthFail = env("HEALTH_FAIL", "false") == "true"
	dsn = env("DATABASE_URL", "postgres://shop:shop@postgres:5432/shop?sslmode=disable")
	port := env("PORT", "8080")

	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/health", metricsMiddleware(http.HandlerFunc(healthHandler)))
	mux.Handle("/order", metricsMiddleware(http.HandlerFunc(orderHandler)))
	mux.Handle("/orders", metricsMiddleware(http.HandlerFunc(ordersHandler)))

	srv := &http.Server{Addr: ":" + port, Handler: mux}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigCh
		log.Println("received SIGTERM, finishing in-flight requests")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("shutdown error: %v", err)
		}
	}()

	log.Printf("api listening on :%s, healthFail=%v", port, healthFail)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
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

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		httpRequestsTotal.WithLabelValues(
			r.Method,
			r.URL.Path,
			strconv.Itoa(rec.status),
		).Inc()
		httpRequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
		).Observe(time.Since(start).Seconds())
	})
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
	_, err = d.Exec(`CREATE TABLE IF NOT EXISTS orders (
		id SERIAL PRIMARY KEY,
		item TEXT NOT NULL,
		processed BOOLEAN NOT NULL DEFAULT FALSE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		d.Close()
		return nil, err
	}
	db = d
	return db, nil
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if healthFail {
		http.Error(w, "unhealthy", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
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
