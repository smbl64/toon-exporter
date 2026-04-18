FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /toon-exporter .

FROM scratch
COPY --from=builder /toon-exporter /toon-exporter
EXPOSE 9101
ENTRYPOINT ["/toon-exporter"]
