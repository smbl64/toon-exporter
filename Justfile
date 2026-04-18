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
    ./scripts/prom-test.sh {{toon}} {{port}}

# Clean build artifacts and test data
clean:
    rm -f toon_exporter
    rm -rf .prom-test
