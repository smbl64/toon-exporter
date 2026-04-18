# Toon API Reference

Notes on the local HTTP API exposed by a rooted Toon thermostat. All endpoints are unauthenticated HTTP GET requests returning JSON unless noted otherwise. The base URL is `http://eneco.lan`.

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
