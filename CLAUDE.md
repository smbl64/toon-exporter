# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o toon_exporter .   # build binary
go test ./...                 # run all tests
go test ./internal/collector/ # run collector tests only
go vet ./...                  # vet
```

With Just:

```bash
just build       # build binary
just test        # run tests
just vet         # go vet
just docker-build  # build Docker image locally
just docker-push   # build and push multi-arch image to Docker Hub
```

## Architecture

Single binary. `main.go` reads three env vars (`TOON_ADDR`, `LISTEN_PORT`, `READ_TIMEOUT`), registers one Prometheus collector, and serves `/metrics`.

All collection logic lives in `internal/collector/`:

- `types.go` — JSON structs for the three Toon API responses
- `toon.go` — `toonCollector` implements `prometheus.Collector`; `Collect()` calls three sub-functions, one per endpoint

**Endpoints polled on every scrape:**

| Function | Toon endpoint | Data |
|---|---|---|
| `collectPowerUsage` | `/happ_pwrusage` | Instant watts, solar watts |
| `collectThermostat` | `/happ_thermstat` | Temperature, setpoint, burner state, modulation, program state |
| `collectZWave` | `/hdrv_zwave` | Smart meter flows (W / L/h) and cumulative totals (Wh / L) |

**Units to keep in mind:**
- `toon_meter_flow{tariff="gas"}` — L/h (as reported by the P1 meter, not computed)
- `toon_meter_total{tariff="gas"}` — litres (divide by 1000 for m³)
- `toon_meter_flow{tariff="lt"/"nt"}` — watts
- Thermostat temperatures come in from the API as integers × 100; `collectThermostat` divides by 100.0

The `tariff` label uses Toon's naming: `lt` = peak, `nt` = off-peak (counterintuitive Dutch P1 convention).

`toon_scrape_success{endpoint}` is emitted by each sub-function — `1` on success, `0` on any HTTP or parse error — so scrape health is observable per endpoint.

## Toon API

`TOON.md` contains detailed documentation of the Toon's local HTTP API: endpoint shapes, field names, units, and observations from live testing. Read it before touching collection logic.
