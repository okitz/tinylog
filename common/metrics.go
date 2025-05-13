package common

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
)

type Metrics struct {
	Timestamp   string  `json:"timestamp"`
	SensorID    string  `json:"sensor_id"`
	Temperature float64 `json:"temperature"`
	Illuminance float64 `json:"illuminance"`
	Status      string  `json:"status"`
}

func RandomMetrics(clientID string) string {
	data := Metrics{
		Timestamp:   time.Now().Format(time.RFC3339),
		SensorID:    clientID,
		Temperature: rand.Float64() * 30,
		Illuminance: rand.Float64() * 100,
		Status:      "OK",
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return ""
	}
	return string(jsonData)
}
