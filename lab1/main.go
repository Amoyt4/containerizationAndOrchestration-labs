package main

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime"
	"strconv"
	"sync/atomic"
)

var keepAlive [][]byte
var burnCounter uint64

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/eat", eatHandler)
	mux.HandleFunc("/burn", burnHandler)

	addr := ":8080"
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func eatHandler(w http.ResponseWriter, r *http.Request) {
	mbStr := r.URL.Query().Get("mb")
	mb, err := strconv.Atoi(mbStr)
	if err != nil || mb <= 0 {
		http.Error(w, "invalid 'mb' parameter, expected positive integer", http.StatusBadRequest)
		return
	}

	const mbSize = 1024 * 1024
	buf := make([]byte, mb*mbSize)
	for i := range buf {
		buf[i] = 0xFF
	}
	keepAlive = append(keepAlive, buf)

	var held int
	for _, b := range keepAlive {
		held += len(b)
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	resp := map[string]any{
		"status":        "ok",
		"allocated_mb":  mb,
		"total_held_mb": held / mbSize,
		"heap_alloc_mb": m.HeapAlloc / mbSize,
		"heap_sys_mb":   m.HeapSys / mbSize,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func burnHandler(w http.ResponseWriter, r *http.Request) {
	done := r.Context().Done()

	go func() {
		for {
			select {
			case <-done:
				return
			default:
				atomic.AddUint64(&burnCounter, 1)
			}
		}
	}()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("burning one core, close the connection to stop\n"))
}
