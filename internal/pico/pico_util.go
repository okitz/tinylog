//go:build pico || pico_w

package pico

import (
	"time"

	"github.com/soypat/cyw43439"
)

func SetupLED(dev *cyw43439.Device, ch <-chan struct{}) {
	// Wait for USB to initialize:
	time.Sleep(time.Second)
	for {
		select {
		case <-ch:
			err := dev.GPIOSet(0, true)
			if err != nil {
				println("err", err.Error())
			} else {
				println("LED ON")
			}
			time.Sleep(500 * time.Millisecond)
			err = dev.GPIOSet(0, false)
			if err != nil {
				println("err", err.Error())
			} else {
				println("LED OFF")
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}
