package max6675

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Max6675Device struct {
	Name       string
	DevicePath string
	Interval   int

	Value float64

	valueAvail  bool
	fh          *os.File
	stopChannel chan bool
}

func NewMax6675(name string, path string, interval int) *Max6675Device {
	return &Max6675Device{
		Name:        name,
		DevicePath:  path,
		Interval:    interval,
		stopChannel: make(chan bool, 1),
	}
}

func (m6 *Max6675Device) Start() error {
	if err := m6.openDevice(); err != nil {
		log.Printf("unable to open %s: %s", m6.DevicePath, err)
		return err
	}

	go func() {
		ticker := time.NewTicker(time.Duration(m6.Interval) * time.Second)
		errors := 0
	m6Loop:
		for {

			select {
			case <-ticker.C:
				if err := m6.readValue(); err != nil {
					log.Printf("unable to get value: %s", err)
					errors++
					if errors > 10 {
						log.Printf("%d errors reading value, exiting read loop", errors)
						break m6Loop
					}
				}
			case <-m6.stopChannel:
				break m6Loop

			}
		}
		ticker.Stop()
		m6.fh.Close()
	}()
	return nil
}

func (m6 *Max6675Device) Stop() {
	m6.stopChannel <- true
}

func (m6 *Max6675Device) openDevice() (err error) {
	m6.fh, err = os.Open(m6.DevicePath)
	return
}

func (m6 *Max6675Device) readValue() (err error) {
	raw := []byte{0, 0}

	n, err := m6.fh.Read(raw)
	if err != nil {
		log.Printf("unable to read value from %s: %v", m6.DevicePath, err)
		return err
	}
	if n != 2 {
		return fmt.Errorf("tried to read 2 bytes, actually read %d", n)
	}
	val := uint16(raw[0])<<8 | uint16(raw[1])
	if val&0x04 == 0x04 {
		m6.valueAvail = false
		return fmt.Errorf("invalid data returned. Marking as unavailable")
	}
	if val&0x8000 == 0x8000 {
		val >>= 3
		val -= 4096
	} else {
		val >>= 3
	}
	m6.Value = float64(val) * .25
	m6.valueAvail = true
	return nil
}

func (m6 *Max6675Device) JsonResponse() map[string]interface{} {
	rv := make(map[string]interface{})
	if m6.valueAvail {
		rv["temp"] = m6.Value
	} else {
		rv["temp"] = "unavailable"
	}
	return rv
}
