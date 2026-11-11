# Toon API Reference

Notes on the local HTTP API exposed by a rooted Toon thermostat. All endpoints are unauthenticated HTTP GET requests returning JSON unless noted otherwise. The base URL is the Toon's local hostname or IP (e.g. `http://toon.local`).

## Endpoints

### Power Usage

```
GET /happ_pwrusage?action=GetCurrentUsage
```

Returns current power draw and solar production. Gas flow is present in the response but always `null` — use the Z-wave endpoint instead.

```json
{
  "result": "ok",
  "powerUsage":      { "value": 324, "avgValue": 249.07 },
  "powerProduction": { "value": 0,   "avgValue": 0.0 },
  "gasUsage":        { "value": null, "avgValue": null }
}
```

- `value` is the current reading in watts; `avgValue` is a rolling average
- `gasUsage.value` is always `null` — gas data comes from the Z-wave endpoint

---

### Thermostat Info

```
GET /happ_thermstat?action=getThermostatInfo
```

Returns the full thermostat state. All numeric values are returned as strings.

```json
{
  "result": "ok",
  "currentTemp": "2034",
  "currentSetpoint": "1500",
  "activeState": "2",
  "burnerInfo": "0",
  "currentModulationLevel": "0",
  "programState": "0",
  "currentInternalBoilerSetpoint": "6",
  "errorFound": "255",
  "connection": "0",
  "otCommError": "0"
}
```

**Important:** temperatures are multiplied by 100. Divide by 100 to get °C:
- `"2034"` → 20.34 °C
- `"1500"` → 15.00 °C

**`activeState` values:**

| Value | Meaning |
|---|---|
| `0` | Comfort |
| `1` | Home |
| `2` | Sleep |
| `3` | Away |
| `-1` | Manual override |

**`burnerInfo` values:**

| Value | Meaning |
|---|---|
| `0` | Off |
| `1` | Heating |
| `2` | Hot water |
| `3` | Pre-heat |

**`currentModulationLevel`:** boiler modulation as a percentage (0–100). Observed hitting 100% when the boiler is working at full capacity (e.g. filling a bath).

---

### Z-wave Devices / Smart Meter

```
GET /hdrv_zwave?action=getDevices.json
```

Returns a flat map of Z-wave devices keyed by internal address. The HAE_METER_v2 (Dutch DSMR P1 smart meter) is always present and provides the authoritative gas and electricity readings.

```json
{
  "dev_3":   { "type": "HAE_METER_v2", "IsConnected": "1", "HealthValue": "10" },
  "dev_3.1": { "type": "gas",               "CurrentGasFlow": "84.00",  "CurrentGasQuantity": "8555590.00" },
  "dev_3.2": { "type": "elec",              "CurrentElectricityFlow": "NaN", "CurrentElectricityQuantity": "NaN" },
  "dev_3.3": { "type": "elec_delivered_nt", "CurrentElectricityFlow": "0.00",   "CurrentElectricityQuantity": "17097942.00" },
  "dev_3.4": { "type": "elec_received_nt",  "CurrentElectricityFlow": "0.00",   "CurrentElectricityQuantity": "0.00" },
  "dev_3.5": { "type": "elec_delivered_lt", "CurrentElectricityFlow": "287.00", "CurrentElectricityQuantity": "15289553.00" },
  "dev_3.6": { "type": "elec_received_lt",  "CurrentElectricityFlow": "0.00",   "CurrentElectricityQuantity": "2.00" }
}
```

**Device types:**

| Type | Description | Flow unit | Total unit |
|---|---|---|---|
| `gas` | Gas meter | L/h | L |
| `elec` | Electricity (combined, often NaN) | W | Wh |
| `elec_delivered_nt` | Grid draw, off-peak tariff | W | Wh |
| `elec_delivered_lt` | Grid draw, peak tariff | W | Wh |
| `elec_received_nt` | Solar feed-back, off-peak | W | Wh |
| `elec_received_lt` | Solar feed-back, peak | W | Wh |

- `nt` = normal tariff (off-peak, typically nights and weekends in NL)
- `lt` = laag tarief... actually the *peak* tariff in the Dutch P1 naming convention (confusingly inverted — `lt` in the API corresponds to the daytime/peak rate)
- `dev_3.2` (`elec`) always returns `NaN` and can be ignored
- All values are strings, even though they look like numbers
- `CurrentGasQuantity` is in **litres** (divide by 1000 for m³)

---

### Boiler Values

```
GET /boilerstatus/boilervalues.txt
```

Returns a plain-text `key=value` file with OpenTherm boiler parameters. **Not available on all setups** — returns 404 if the Toon is not connected to a boiler via OpenTherm (e.g. when using a zone valve controller instead). The exporter handles this gracefully.

---

## Observations from live testing

- The smart meter gas flow (`CurrentGasFlow`) updates approximately every 5–10 seconds on the device side but the Toon may cache it briefly.
- Boiler modulation (`currentModulationLevel`) can jump from 0 to 100% within a single scrape interval when hot water demand is high.
- `burnerInfo=2` (hot water) is distinct from `burnerInfo=1` (space heating) — useful for separating hot water demand from heating demand in Grafana.
- Power draw from the oven was observed stepping: ~325W idle → 2265W (one element) → 5033W (both elements) → ~3150W (one element cycling off at target temp).
- The `happ_pwrusage` endpoint's `powerUsage.value` matches `elec_delivered_lt` flow from the Z-wave endpoint during peak hours, confirming they read from the same P1 meter.

---

## Control / write API (live-validated)

The read endpoints above drive the Prometheus exporter. The same local HTTP API also exposes a small set of **write** actions, all of which are **unauthenticated `GET` requests** — so *any* host that can reach the Toon's LAN address can command it, with no token or special client. This is both the enabler for remote control and its main security caveat.

Base: `http://<TOON_HOST>` (port 80 by default; e.g. `http://toon.local`). No authentication, no special headers required.

### The write actions

| # | Action | HTTP request | Effect |
|---|--------|--------------|--------|
| 1 | Set setpoint | `GET /happ_thermstat?action=setSetpoint&Setpoint=<°C × 100>` | Set target temperature; forces **manual override** (`activeState` → `-1`) |
| 2 | Change program | `GET /happ_thermstat?action=changeSchemeState&state=<n>&temperatureState=<m>` | Switch active program (Comfort / Home / Sleep / Away) / schedule on-off |
| 3 | Manual override | `GET /happ_thermstat?action=setSetpoint&Setpoint=<current×100>` | Re-fire current setpoint → `activeState` = `-1` (Manual), schedule no longer applies |

- **`Setpoint` scaling is ×100** (hundredths of a degree), the same convention as the read side. 21.5 °C → `Setpoint=2150`; 15.0 °C → `Setpoint=1500`.
- **`temperatureState` 0–3** = **Comfort / Home / Sleep / Away** (directly analogous to the `activeState` read field). **`state`** is a secondary parameter, **commonly `2`**; its exact slot/mode semantics aren't fully pinned down in the community sources, but holding it at `2` and varying `temperatureState` 0–3 reliably drives the four programs — which is all that's needed for remote control.

### Setpoint scaling — confirmed live

One older community note suggested **×10** (`Setpoint=60 → 6 °C`). The **live read-back confirmed ×100** (see validation below), settling the ambiguity in favour of ×100 — consistent with the read API and the Home Assistant / Domoticz integrations.

### Live validation

A full write→read-back cycle was run against a rooted Toon reachable on the local network (no exporter changes):

| Phase | Request | Read-back | Result |
|---|---|---|---|
| Baseline | `getThermostatInfo` | `currentSetpoint=1500` (15.00 °C), `activeState=2` (Sleep), burner off | original state |
| **Set setpoint** | `setSetpoint&Setpoint=1975` | **`currentSetpoint=1975` (19.75 °C)**, `activeState=-1` (Manual) | ✅ setpoint confirmed, ×100 confirmed |
| **Change program** | `changeSchemeState&state=2&temperatureState=1` (Home) | accepted `{"result":"ok"}` | ✅ program write accepted |
| Restore | `changeSchemeState&state=2&temperatureState=2` + `setSetpoint&Setpoint=1500` | `currentSetpoint=1500` (15.00 °C), `activeState=2` (Sleep) | ✅ restored to original |

- The **setpoint write is definitively confirmed**: 1975 in → 1975 out (19.75 °C), which also proves **×100 scaling**.
- The **program/scheme write is accepted** (`ok`); its `temperatureState` 0–3 mapping matches the `activeState` field and the Home Assistant (`cyberjunky/home-assistant-toon_climate`) / Domoticz implementations.
- The device was **restored to its original state** (Sleep program, 15.00 °C) after testing — no lasting changes.

### Control-path caveats

- **Unauthenticated write surface.** Every control action is a plain unauthenticated `GET` on the LAN. On an open or internet-exposed network, anyone who can reach the Toon's address can change its state. **If exposing control to the internet, front it with a token / ACL** (e.g. a Tailscale tailnet ACL, a bearer-token wrapper, or a Home-Assistant-level user) so only authorised clients can issue the `GET`s.
- **`setSetpoint` forces manual override.** Driving a setpoint via the write flips `activeState` to `-1` (Manual); the schedule no longer governs the setpoint until `changeSchemeState` is fired.
- **Setpoint scaling is ×100** (hundredths of a degree). The live read-back confirmed ×100; the isolated older ×10 note is overridden by that live evidence.
- **`changeSchemeState` `state` parameter** (commonly `2`) selects a scheme slot / mode. Its exact semantics are not fully pinned down, but keeping it at `2` and varying `temperatureState` 0–3 drives the Comfort / Home / Sleep / Away program reliably.

### Remote-access architecture (how to reach it from outside)

To control the Toon from outside the LAN, reach it and fire the two `GET`s:

1. **Tailscale / WireGuard tailnet (recommended, simplest).** Assign the Toon host (or a gateway / exporter box) a stable `*.ts.net` name; from any tailnet member (phone, laptop, VPS), fire: `http://toon.tailnet.ts.net/happ_thermstat?action=setSetpoint&Setpoint=2150`. **No inbound ports**, per-device identity, and the unauthenticated API stays private to the tailnet. Add a tailnet ACL (or a light bearer token) so only your devices can reach it.
2. **Authenticated gateway / small write endpoint (cleanest product shape).** Add a ~30-line write endpoint to the exporter (or a tiny Flask / FastAPI sidecar) that proxies `?temp=21.5` and `?program=home` to the two `GET`s. Reach that gateway over Tailscale (or a reverse proxy through a VPS). This is the intended **next step** for producing the actual remote-controller product.
3. **Home Automation bridge (pragmatic, no custom code).** Wrap the two `GET`s in **Home Assistant** (`cyberjunky/home-assistant-toon_climate`) or **Node-RED**, listening on the LAN / tailnet, translating "set to 21.5 °C" / "switch to Sleep" into the two `GET`s. Gives you buttons, automations, and a safe boundary with zero custom HTTP code.

The cleanest homelab path is **#1 + #2**: a Tailscale name on the Toon (or the exporter box) plus a tiny authenticated write endpoint that proxies the two `GET`s — reachable from any Tailscale member, protected by a token / tailnet ACL.
