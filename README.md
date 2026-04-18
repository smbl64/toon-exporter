# toon-exporter

[![Docker Pulls](https://img.shields.io/docker/pulls/smbl64/toon-exporter)](https://hub.docker.com/r/smbl64/toon-exporter)
[![Docker Image Size](https://img.shields.io/docker/image-size/smbl64/toon-exporter/latest)](https://hub.docker.com/r/smbl64/toon-exporter)
[![Go Version](https://img.shields.io/github/go-mod/go-version/smbl64/toon-exporter)](go.mod)
[![License](https://img.shields.io/github/license/smbl64/toon-exporter)](LICENSE)

A Prometheus exporter for the [Toon thermostat](https://www.eneco.nl/toon/) by Eneco, written in Go.

Polls the Toon's local HTTP API and exposes temperature, boiler state, power draw, solar production, and smart meter readings as Prometheus metrics at `/metrics`.

> **Prerequisite:** A rooted Toon reachable on your local network.

---

## Quick Start

```bash
# Replace http://toon.local with your Toon's actual address
docker run -e TOON_ADDR=http://toon.local -p 9100:9100 smbl64/toon-exporter:latest
```

Metrics are available at `http://localhost:9100/metrics`.

---

## Running

### Docker (recommended)

```bash
# Pull from Docker Hub (amd64 + arm64)
# Replace http://toon.local with your Toon's actual address
docker run -e TOON_ADDR=http://toon.local -p 9100:9100 smbl64/toon-exporter:latest
```

### Docker Compose

```bash
docker compose up -d
```

> If your host can resolve your Toon's hostname via mDNS, switch to `network_mode: host` in `docker-compose.yml` to avoid DNS issues inside the container.

### From source

```bash
go build -o toon-exporter .
TOON_ADDR=http://toon.local ./toon-exporter  # replace with your Toon's actual address
```

### With Just

```bash
just build    # build binary
just run      # build and run against the real Toon
just test     # run tests
just vet      # run go vet
```

---

## Environment Variables

| Variable       | Default            | Description                         |
|----------------|--------------------|-------------------------------------|
| `TOON_ADDR`    | `http://toon.local` (example) | Base URL of the Toon device  |
| `LISTEN_PORT`  | `:9100`            | Address/port to listen on           |
| `READ_TIMEOUT` | `10s`              | HTTP timeout for Toon API requests  |

---

## Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: toon
    static_configs:
      - targets: ["localhost:9100"]
```

---

## Metrics

<details>
<summary><strong>Thermostat</strong></summary>

| Metric                              | Type  | Description                                                   |
|-------------------------------------|-------|---------------------------------------------------------------|
| `toon_temperature_current_celsius`  | Gauge | Current room temperature                                      |
| `toon_temperature_setpoint_celsius` | Gauge | Target setpoint                                               |
| `toon_active_state`                 | Gauge | Program state: `0`=comfort, `1`=home, `2`=sleep, `3`=away, `-1`=manual |
| `toon_burner_state`                 | Gauge | Boiler state: `0`=off, `1`=on, `2`=hot water, `3`=pre-heat   |
| `toon_modulation_level`             | Gauge | Boiler modulation percentage                                  |

</details>

<details>
<summary><strong>Power</strong></summary>

| Metric                      | Type  | Description                          |
|-----------------------------|-------|--------------------------------------|
| `toon_power_usage_watts`    | Gauge | Current power draw (W)               |
| `toon_power_production_watts` | Gauge | Current solar production (W)       |

</details>

<details>
<summary><strong>Smart Meter</strong></summary>

| Metric            | Type    | Labels                | Description                                                         |
|-------------------|---------|-----------------------|---------------------------------------------------------------------|
| `toon_meter_flow` | Gauge   | `tariff`, `direction` | Current flow — electricity in W, gas in L/h                        |
| `toon_meter_total`| Counter | `tariff`, `direction` | Cumulative reading — electricity in Wh, gas in L                   |

**Label values:**

- `tariff`: `lt` (peak), `nt` (off-peak), `gas`
- `direction`: `delivered` (consumed / drawn from grid), `received` (fed back / solar export)

</details>

<details>
<summary><strong>Scrape health</strong></summary>

| Metric                | Type  | Labels     | Description                                         |
|-----------------------|-------|------------|-----------------------------------------------------|
| `toon_scrape_success` | Gauge | `endpoint` | `1` if the last scrape of the endpoint succeeded, `0` otherwise |

Endpoints: `pwrusage`, `thermstat`, `zwave`.

</details>

---

## Grafana Example Queries

**Current room temperature**
```promql
toon_temperature_current_celsius
```

**Temperature vs setpoint**
```promql
toon_temperature_current_celsius
toon_temperature_setpoint_celsius
```

**Boiler is actively heating**
```promql
toon_burner_state == 1
```

**Boiler modulation level over time**
```promql
toon_modulation_level
```

**Net power draw (consumption minus solar)**
```promql
toon_power_usage_watts - toon_power_production_watts
```

**Total electricity consumed today (peak + off-peak, in kWh)**
```promql
(
  increase(toon_meter_total{tariff="lt", direction="delivered"}[24h])
  + increase(toon_meter_total{tariff="nt", direction="delivered"}[24h])
) / 1000
```

**Total electricity fed back to grid today (in kWh)**
```promql
(
  increase(toon_meter_total{tariff="lt", direction="received"}[24h])
  + increase(toon_meter_total{tariff="nt", direction="received"}[24h])
) / 1000
```

**Current gas flow (L/h)**
```promql
toon_meter_flow{tariff="gas", direction="delivered"}
```

**All scrape endpoints healthy**
```promql
toon_scrape_success
```

---

## How It Works

The exporter polls three endpoints on the Toon's local HTTP API on every Prometheus scrape:

| Internal endpoint | Data collected |
|---|---|
| `/happ_thermstat` | Temperature, setpoint, burner state, modulation, program state |
| `/happ_pwrusage`  | Instant power draw and solar production |
| `/hdrv_zwave`     | Smart meter flows and cumulative readings |

No authentication is required. The Toon must be reachable on the local network.
