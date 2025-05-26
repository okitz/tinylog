package testutil

import (
	"fmt"
	"math/rand/v2"
	"time"

	log_v1 "github.com/okitz/mqtt-log-pipeline/api/log"
)

func RandomMetricsJson(clientID string) string {
	data := log_v1.Metrics{
		Timestamp:   time.Now().Format(time.RFC3339),
		SensorId:    clientID,
		Temperature: rand.Float64() * 30,
		Illuminance: rand.Float64() * 100,
		Status:      "OK",
	}
	jsonData, err := data.MarshalJSON()
	if err != nil {
		fmt.Println("Error marshalling JSON:", err)
		return ""
	}
	return string(jsonData)
}
