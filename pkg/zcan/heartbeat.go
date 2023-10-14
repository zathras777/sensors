package zcan

import (
	"time"

	"go.einride.tech/can"
)

func (dev *ZehnderDevice) makeHeartbeatFrame() can.Frame {
	id := uint32(0x10000000 + uint32(dev.NodeID))
	return can.Frame{ID: id, IsExtended: true}
}

func (dev *ZehnderDevice) heartbeat() {
	dev.wg.Add(1)

	if dev.hasNetwork() {
		dev.txQ <- dev.makeHeartbeatFrame()
	}
	timer := time.NewTicker(2 * time.Second)

loop:
	for {
		select {
		case frame := <-dev.heartbeatQ:
			if frame.IsRemote {
				nodeId := frame.ID & 0x3F
				if nodeId == uint32(dev.NodeID) {
					if dev.hasNetwork() {
						dev.txQ <- dev.makeHeartbeatFrame()
					}
					timer.Reset(2 * time.Second)
				}
			}
		case <-dev.stopSignal:
			break loop
		case <-timer.C:
			if dev.hasNetwork() {
				dev.txQ <- dev.makeHeartbeatFrame()
			}
		}
	}
	timer.Stop()
	dev.wg.Done()
}
