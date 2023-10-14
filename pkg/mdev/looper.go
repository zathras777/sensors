package mdev

import (
	"log"
	"time"
)

var interval time.Duration

func (md *ModbusDevice) Start(intval int) {
	interval = time.Duration(intval) * time.Second
	if md.stopper != nil {
		md.stopper <- true
	}
	md.stopper = make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(interval)
		errors := 0
	TickerLoop:
		for {
			select {
			case <-ticker.C:
				if err := md.ReadOnce(); err != nil {
					errors++
					if errors > 5 {
						log.Printf("unable to read data repeatedly. Aborting collection loop")
						break TickerLoop
					}
				}
			case <-md.stopper:
				break TickerLoop
			}
		}
		ticker.Stop()
		md.stopper = nil
	}()
}

func (md *ModbusDevice) Stop() {
	if md.stopper == nil {
		return
	}
	md.stopper <- true
}
