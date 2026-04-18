package collector

import "encoding/json"

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
