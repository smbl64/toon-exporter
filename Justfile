image := "smbl64/toon-exporter"
toon  := "http://eneco.lan"
port  := "9100"

# List available recipes
default:
    @just --list

# Build the binary locally
build:
    go build -o toon_exporter .

# Run locally against the real Toon
run: build
    ./toon_exporter -toon-addr={{toon}} -listen-addr=:{{port}}

# Run tests
test:
    go test ./...

# Run go vet
vet:
    go vet ./...

# Build Docker image locally (current platform)
docker-build:
    docker build -t {{image}}:latest .

# Build and push multi-arch image to Docker Hub
docker-push:
    docker buildx build --platform linux/amd64,linux/arm64 -t {{image}}:latest --push .

# Pull fresh image on the host (useful after a push)
docker-pull:
    docker pull {{image}}:latest

# Start a throwaway Prometheus pointed at the local exporter for testing
prom-test: build
    #!/usr/bin/env bash
    set -e
    mkdir -p .prom-test
    cat > .prom-test/prometheus.yml <<EOF
    global:
      scrape_interval: 10s
    scrape_configs:
      - job_name: toon
        static_configs:
          - targets: ["host.docker.internal:{{port}}"]
    EOF
    HOST_IP=$(ipconfig getifaddr en0)
    ./toon_exporter -toon-addr={{toon}} -listen-addr=:{{port}} &
    EXPORTER_PID=$!
    echo "Exporter running (PID $EXPORTER_PID)"
    podman run --rm \
        --add-host host.docker.internal:$HOST_IP \
        -p 19090:9090 \
        -v "$(pwd)/.prom-test:/etc/prometheus:z" \
        prom/prometheus \
        --config.file=/etc/prometheus/prometheus.yml \
        --storage.tsdb.path=/tmp/tsdb \
        --storage.tsdb.retention.time=1h \
        --log.level=warn
    kill $EXPORTER_PID 2>/dev/null
    rm -rf .prom-test

# Clean build artifacts and test data
clean:
    rm -f toon_exporter
    rm -rf .prom-test
