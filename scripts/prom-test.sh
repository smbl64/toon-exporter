#!/usr/bin/env bash
set -euo pipefail

TOON=${1:-http://eneco.lan}
PORT=${2:-9100}
IMAGE="smbl64/toon-exporter"

mkdir -p .prom-test
cat > .prom-test/prometheus.yml <<EOF
global:
  scrape_interval: 10s
scrape_configs:
  - job_name: toon
    static_configs:
      - targets: ["host.docker.internal:${PORT}"]
EOF

HOST_IP=$(ipconfig getifaddr en0)

./toon_exporter -toon-addr="$TOON" -listen-addr=":$PORT" &
EXPORTER_PID=$!
echo "Exporter running (PID $EXPORTER_PID)"

cleanup() {
    echo "Shutting down..."
    kill "$EXPORTER_PID" 2>/dev/null
    rm -rf .prom-test
}
trap cleanup EXIT

podman run --rm \
    --add-host "host.docker.internal:$HOST_IP" \
    -p 19090:9090 \
    -v "$(pwd)/.prom-test:/etc/prometheus:z" \
    prom/prometheus \
    --config.file=/etc/prometheus/prometheus.yml \
    --storage.tsdb.path=/tmp/tsdb \
    --storage.tsdb.retention.time=1h \
    --log.level=warn
