package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/smbl64/toon-exporter/internal/collector"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Printf("invalid %s value, using default %s", key, def)
	}
	return def
}

func main() {
	toonAddr := getEnv("TOON_ADDR", "http://eneco.lan")
	listenAddr := getEnv("LISTEN_PORT", ":9100")
	timeout := parseDurationEnv("READ_TIMEOUT", 10*time.Second)

	coll := collector.NewToonCollector(collector.CollectorConfig{
		ToonAddr: toonAddr,
		Timeout:  timeout,
	})
	prometheus.MustRegister(coll)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<html><body><a href="/metrics">metrics</a></body></html>`)
	})

	log.Printf("toon-exporter listening on %s (toon: %s)", listenAddr, toonAddr)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
