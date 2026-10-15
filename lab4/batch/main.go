package main

import (
	"log"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

var keepAlive [][]byte
var burnCounter uint64

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	mbPerTick, _ := strconv.Atoi(env("MB_PER_TICK", "10"))
	tickInterval, _ := time.ParseDuration(env("TICK_INTERVAL", "5s"))
	log.Printf("batch started: alloc %d MB every %s", mbPerTick, tickInterval)

	go func() {
		for {
			atomic.AddUint64(&burnCounter, 1)
		}
	}()
	const mbSize = 1024 * 1024
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	for range ticker.C {
		buf := make([]byte, mbPerTick*mbSize)
		for i := range buf {
			buf[i] = 0xFF
		}
		keepAlive = append(keepAlive, buf)

		var totalMB int
		for _, b := range keepAlive {
			totalMB += len(b) / mbSize
		}
		log.Printf("batch: allocated %d MB, total held %d MB", mbPerTick, totalMB)
	}
}
