package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// --- Config ---

type config struct {
	toonAddr   string
	listenAddr string
	timeout    time.Duration
}

// --- API response types ---

type powerValueField struct {
	Value *float64 `json:"value"` // pointer to handle null
}

type PowerUsageResponse struct {
	PowerUsage      powerValueField `json:"powerUsage"`
	PowerProduction powerValueField `json:"powerProduction"`
	GasUsage        powerValueField `json:"gasUsage"`
}

type ThermostatInfo struct {
	CurrentTemp            json.Number `json:"currentTemp"`
	CurrentSetpoint        json.Number `json:"currentSetpoint"`
	ActiveState            json.Number `json:"activeState"`
	BurnerInfo             json.Number `json:"burnerInfo"`
	CurrentModulationLevel json.Number `json:"currentModulationLevel"`
}

// ZWave device fields we care about — all string-encoded floats.
type ZWaveDevice struct {
	Type string `json:"type"`

	// Gas meter (type=gas)
	CurrentGasFlow     string `json:"CurrentGasFlow"`
	CurrentGasQuantity string `json:"CurrentGasQuantity"`

	// Electricity meters (type=elec_delivered_nt/lt, elec_received_nt/lt)
	CurrentElectricityFlow     string `json:"CurrentElectricityFlow"`
	CurrentElectricityQuantity string `json:"CurrentElectricityQuantity"`
}

// --- Collector ---

type toonCollector struct {
	cfg    config
	client *http.Client

	// thermostat
	tempCurrent     *prometheus.Desc
	tempSetpoint    *prometheus.Desc
	burnerState     *prometheus.Desc
	modulationLevel *prometheus.Desc
	activeState     *prometheus.Desc

	// power (from happ_pwrusage — quick/averaged values)
	powerUsage      *prometheus.Desc
	powerProduction *prometheus.Desc

	// smart meter — flows (gauges, watts or L/h)
	meterFlow *prometheus.Desc
	// smart meter — cumulative quantities (counters, Wh or L)
	meterTotal *prometheus.Desc

	scrapeSuccess *prometheus.Desc
}

func newToonCollector(cfg config) *toonCollector {
	tariffLabel := []string{"tariff"} // "nt" (off-peak) or "lt" (peak)
	dirLabel := []string{"direction"} // "delivered" or "received"

	return &toonCollector{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.timeout},

		tempCurrent: prometheus.NewDesc(
			"toon_temperature_current_celsius",
			"Current room temperature in degrees Celsius.",
			nil, nil,
		),
		tempSetpoint: prometheus.NewDesc(
			"toon_temperature_setpoint_celsius",
			"Target temperature setpoint in degrees Celsius.",
			nil, nil,
		),
		burnerState: prometheus.NewDesc(
			"toon_burner_state",
			"Boiler burner state (0=off, 1=on, 2=hot water, 3=pre-heat).",
			nil, nil,
		),
		modulationLevel: prometheus.NewDesc(
			"toon_modulation_level",
			"Boiler modulation level as a percentage.",
			nil, nil,
		),
		activeState: prometheus.NewDesc(
			"toon_active_state",
			"Active program state (0=comfort, 1=home, 2=sleep, 3=away, -1=manual).",
			nil, nil,
		),
		powerUsage: prometheus.NewDesc(
			"toon_power_usage_watts",
			"Current power draw in watts (from happ_pwrusage).",
			nil, nil,
		),
		powerProduction: prometheus.NewDesc(
			"toon_power_production_watts",
			"Current solar power production in watts (from happ_pwrusage).",
			nil, nil,
		),
		meterFlow: prometheus.NewDesc(
			"toon_meter_flow",
			"Current flow from the smart meter. Electricity in watts, gas in litres/hour.",
			append(tariffLabel, dirLabel...),
			prometheus.Labels{"unit": "mixed"},
		),
		meterTotal: prometheus.NewDesc(
			"toon_meter_total",
			"Cumulative meter reading. Electricity in Wh, gas in litres.",
			append(tariffLabel, dirLabel...),
			prometheus.Labels{"unit": "mixed"},
		),
		scrapeSuccess: prometheus.NewDesc(
			"toon_scrape_success",
			"Whether the last scrape of this endpoint succeeded (1=success, 0=failure).",
			[]string{"endpoint"}, nil,
		),
	}
}

func (c *toonCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.tempCurrent
	ch <- c.tempSetpoint
	ch <- c.burnerState
	ch <- c.modulationLevel
	ch <- c.activeState
	ch <- c.powerUsage
	ch <- c.powerProduction
	ch <- c.meterFlow
	ch <- c.meterTotal
	ch <- c.scrapeSuccess
}

func (c *toonCollector) Collect(ch chan<- prometheus.Metric) {
	c.collectPowerUsage(ch)
	c.collectThermostat(ch)
	c.collectZWave(ch)
}

func (c *toonCollector) fetchJSON(url string, target any) error {
	resp, err := c.client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (c *toonCollector) collectPowerUsage(ch chan<- prometheus.Metric) {
	const endpoint = "pwrusage"
	url := c.cfg.toonAddr + "/happ_pwrusage?action=GetCurrentUsage"

	var data PowerUsageResponse
	if err := c.fetchJSON(url, &data); err != nil {
		log.Printf("error scraping %s: %v", endpoint, err)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, endpoint)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, endpoint)

	if data.PowerUsage.Value != nil {
		ch <- prometheus.MustNewConstMetric(c.powerUsage, prometheus.GaugeValue, *data.PowerUsage.Value)
	}
	if data.PowerProduction.Value != nil {
		ch <- prometheus.MustNewConstMetric(c.powerProduction, prometheus.GaugeValue, *data.PowerProduction.Value)
	}
}

func (c *toonCollector) collectThermostat(ch chan<- prometheus.Metric) {
	const endpoint = "thermstat"
	url := c.cfg.toonAddr + "/happ_thermstat?action=getThermostatInfo"

	var data ThermostatInfo
	if err := c.fetchJSON(url, &data); err != nil {
		log.Printf("error scraping %s: %v", endpoint, err)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, endpoint)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, endpoint)

	if v, err := data.CurrentTemp.Float64(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.tempCurrent, prometheus.GaugeValue, v/100.0)
	}
	if v, err := data.CurrentSetpoint.Float64(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.tempSetpoint, prometheus.GaugeValue, v/100.0)
	}
	if v, err := data.ActiveState.Float64(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.activeState, prometheus.GaugeValue, v)
	}
	if v, err := data.BurnerInfo.Float64(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.burnerState, prometheus.GaugeValue, v)
	}
	if v, err := data.CurrentModulationLevel.Float64(); err == nil {
		ch <- prometheus.MustNewConstMetric(c.modulationLevel, prometheus.GaugeValue, v)
	}
}

// collectZWave reads the HAE_METER_v2 smart meter data from the Z-wave endpoint.
// It emits toon_meter_flow and toon_meter_total with tariff/direction labels.
// Gas is reported in L/h (flow) and L (total); electricity in W and Wh.
func (c *toonCollector) collectZWave(ch chan<- prometheus.Metric) {
	const endpoint = "zwave"
	url := c.cfg.toonAddr + "/hdrv_zwave?action=getDevices.json"

	var devices map[string]ZWaveDevice
	if err := c.fetchJSON(url, &devices); err != nil {
		log.Printf("error scraping %s: %v", endpoint, err)
		ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 0, endpoint)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.scrapeSuccess, prometheus.GaugeValue, 1, endpoint)

	for _, dev := range devices {
		switch dev.Type {
		case "gas":
			if f, ok := parseFloat(dev.CurrentGasFlow); ok {
				ch <- prometheus.MustNewConstMetric(c.meterFlow, prometheus.GaugeValue, f, "gas", "delivered")
			}
			if f, ok := parseFloat(dev.CurrentGasQuantity); ok {
				ch <- prometheus.MustNewConstMetric(c.meterTotal, prometheus.CounterValue, f, "gas", "delivered")
			}

		case "elec_delivered_nt":
			if f, ok := parseFloat(dev.CurrentElectricityFlow); ok {
				ch <- prometheus.MustNewConstMetric(c.meterFlow, prometheus.GaugeValue, f, "nt", "delivered")
			}
			if f, ok := parseFloat(dev.CurrentElectricityQuantity); ok {
				ch <- prometheus.MustNewConstMetric(c.meterTotal, prometheus.CounterValue, f, "nt", "delivered")
			}

		case "elec_delivered_lt":
			if f, ok := parseFloat(dev.CurrentElectricityFlow); ok {
				ch <- prometheus.MustNewConstMetric(c.meterFlow, prometheus.GaugeValue, f, "lt", "delivered")
			}
			if f, ok := parseFloat(dev.CurrentElectricityQuantity); ok {
				ch <- prometheus.MustNewConstMetric(c.meterTotal, prometheus.CounterValue, f, "lt", "delivered")
			}

		case "elec_received_nt":
			if f, ok := parseFloat(dev.CurrentElectricityFlow); ok {
				ch <- prometheus.MustNewConstMetric(c.meterFlow, prometheus.GaugeValue, f, "nt", "received")
			}
			if f, ok := parseFloat(dev.CurrentElectricityQuantity); ok {
				ch <- prometheus.MustNewConstMetric(c.meterTotal, prometheus.CounterValue, f, "nt", "received")
			}

		case "elec_received_lt":
			if f, ok := parseFloat(dev.CurrentElectricityFlow); ok {
				ch <- prometheus.MustNewConstMetric(c.meterFlow, prometheus.GaugeValue, f, "lt", "received")
			}
			if f, ok := parseFloat(dev.CurrentElectricityQuantity); ok {
				ch <- prometheus.MustNewConstMetric(c.meterTotal, prometheus.CounterValue, f, "lt", "received")
			}
		}
	}
}

// parseFloat parses a string float, returning (value, true) or (0, false) on error/NaN.
func parseFloat(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) {
		return 0, false
	}
	return f, true
}

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
	cfg := config{
		toonAddr:   getEnv("TOON_ADDR", "http://eneco.lan"),
		listenAddr: getEnv("LISTEN_PORT", ":9100"),
		timeout:    parseDurationEnv("READ_TIMEOUT", 10*time.Second),
	}

	collector := newToonCollector(cfg)
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<html><body><a href="/metrics">metrics</a></body></html>`)
	})

	log.Printf("toon-exporter listening on %s (toon: %s)", cfg.listenAddr, cfg.toonAddr)
	if err := http.ListenAndServe(cfg.listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
