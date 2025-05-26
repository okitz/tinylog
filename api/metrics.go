package metrics

import (
	"fmt"
	"math/rand/v2"
	"time"
)

func RandomMetricsJson(clientID string) string {
	data := Metrics{
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
