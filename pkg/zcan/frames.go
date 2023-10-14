package zcan

import (
	"fmt"
	"log"
)

func (dev *ZehnderDevice) processFrame() {
	dev.wg.Add(1)
	defer dev.wg.Done()

loop:
	for {
		select {
		case frame := <-dev.frameQ:
			if dev.doCapture {
				dev.captureFh.WriteString(fmt.Sprintf("%s\n", frame))
			}
			ck := frame.ID >> 24
			switch ck {
			case 0:
				dev.pdoQ <- frame
			case 0x1F:
				dev.rmiQ <- frame
			case 0x10:
				dev.heartbeatQ <- frame
			default:
				log.Printf("Unknown frame MSB: %02X", ck)
			}
		case <-dev.stopSignal:
			break loop
		}
	}

	if dev.doCapture {
		dev.captureFh.Close()
	}
}
