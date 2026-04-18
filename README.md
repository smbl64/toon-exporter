# toon-exporter

A Prometheus exporter for the [Toon thermostat](https://www.eneco.nl/toon/) by Eneco, written in Go. Requires a rooted Toon accessible on the local network.

## Metrics

| Metric | Type | Description |
|---|---|---|
| `toon_temperature_current_celsius` | Gauge | Current room temperature |
| `toon_temperature_setpoint_celsius` | Gauge | Target setpoint |
| `toon_active_state` | Gauge | Program state: 0=comfort, 1=home, 2=sleep, 3=away, -1=manual |
| `toon_burner_state` | Gauge | Boiler burner: 0=off, 1=on, 2=hot water, 3=pre-heat |
| `toon_modulation_level` | Gauge | Boiler modulation percentage |
| `toon_power_usage_watts` | Gauge | Current power draw (W) |
| `toon_power_production_watts` | Gauge | Current solar production (W) |
| `toon_meter_flow` | Gauge | Current flow from smart meter — electricity (W) or gas (L/h) |
| `toon_meter_total` | Counter | Cumulative meter reading — electricity (Wh) or gas (L) |
| `toon_scrape_success` | Gauge | 1 if last scrape of the endpoint succeeded, 0 if not |

### Labels on `toon_meter_flow` and `toon_meter_total`

- `tariff`: `lt` (peak), `nt` (off-peak), or `gas`
- `direction`: `delivered` (consumed/drawn) or `received` (fed back to grid / solar)

## Running

### Directly

```bash
go build -o toon-exporter .
./toon-exporter
```

Metrics available at `http://localhost:9100/metrics`.

### Docker

```bash
docker build -t toon-exporter .
docker run -p 9100:9100 toon-exporter
```

### Docker Compose

Edit `docker-compose.yml` and set the correct IP for `eneco.lan` under `extra_hosts`, then:

```bash
docker compose up -d
```

> If your host machine can already resolve `eneco.lan` via DNS/mDNS, switch to `network_mode: host` in `docker-compose.yml` instead of using `extra_hosts`.

## Flags

| Flag | Default | Description |
|---|---|---|
| `-toon-addr` | `http://eneco.lan` | Base URL of the Toon device |
| `-listen-addr` | `:9100` | Address to listen on |
| `-timeout` | `10s` | HTTP timeout for Toon API requests |

## Prometheus scrape config

```yaml
scrape_configs:
  - job_name: toon
    static_configs:
      - targets: ["localhost:9100"]
```

## Requirements

- Rooted Toon thermostat on the local network, reachable at `http://eneco.lan`
- No authentication required
