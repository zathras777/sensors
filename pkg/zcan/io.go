package zcan

import (
	"context"
	"fmt"
)

func (dev *ZehnderDevice) receiver() {
	err := dev.connection.open()
	if err != nil {
		fmt.Println(err)
		return
	}

	recv := dev.connection.getReceiver()
	for recv.Receive() {
		frame := recv.Frame()
		dev.frameQ <- frame
	}
	dev.connection.close()
}

func (dev *ZehnderDevice) transmitter() {
	err := dev.connection.open()
	if err != nil {
		fmt.Println(err)
		return
	}

	dev.wg.Add(1)

	tx := dev.connection.getTransmitter()
loop:
	for {
		select {
		case frame := <-dev.txQ:
			tx.TransmitFrame(context.Background(), frame)
		case <-dev.stopSignal:
			break loop
		}
	}
	dev.connection.close()
	dev.wg.Done()
}
