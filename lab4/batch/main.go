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
	maxMB, _ := strconv.Atoi(env("MAX_MB", "200"))
	enableCPU, _ := strconv.ParseBool(env("ENABLE_CPU_BURN", "false"))
	cpuDuty, _ := strconv.Atoi(env("CPU_BURN_PERCENT", "10"))

	log.Printf("batch started: alloc %d MB every %s, max %d MB, cpuBurn=%v, cpuDuty=%d%%",
		mbPerTick, tickInterval, maxMB, enableCPU, cpuDuty)

	if enableCPU {
		burnDur := time.Duration(cpuDuty) * time.Millisecond
		sleepDur := time.Duration(100-cpuDuty) * time.Millisecond
		go func() {
			for {
				end := time.Now().Add(burnDur)
				for time.Now().Before(end) {
					atomic.AddUint64(&burnCounter, 1)
				}
				time.Sleep(sleepDur)
			}
		}()
	}

	const mbSize = 1024 * 1024
	ticker := time.NewTicker(tickInterval)
	defer ticker.Stop()

	var totalMB int
	for range ticker.C {
		if totalMB >= maxMB {
			log.Printf("batch: at max (%d MB), idle", totalMB)
			continue
		}
		buf := make([]byte, mbPerTick*mbSize)
		for i := range buf {
			buf[i] = 0xFF
		}
		keepAlive = append(keepAlive, buf)
		totalMB += mbPerTick
		log.Printf("batch: allocated %d MB, total held %d MB", mbPerTick, totalMB)
	}
}
