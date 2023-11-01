package zcan

import (
	"context"
	"fmt"
	"log"
	"net"

	"go.einride.tech/can/pkg/candevice"
	"go.einride.tech/can/pkg/socketcan"
)

type zConnection struct {
	interfaceName string
	device        *candevice.Device
	counter       int
	prevState     bool
	conn          net.Conn
}

func (conn *zConnection) open_device(interfaceName string) error {
	conn.interfaceName = interfaceName
	d, err := candevice.New(conn.interfaceName)
	if err != nil {
		return err
	}
	var br uint32
	br, err = d.Bitrate()
	if err != nil {
		return err
	}
	if br != 50000 {
		err = d.SetBitrate(50000)
		if err != nil {
			return err
		}
	}
	conn.prevState, err = d.IsUp()
	if err != nil {
		return err
	}
	if !conn.prevState {
		err = d.SetUp()
		if err != nil {
			log.Printf("unable to bring %s interface UP. %v", interfaceName, err)
			return err
		}
	}
	conn.device = d
	return nil
}

func (conn *zConnection) close_device() error {
	if !conn.prevState {
		return conn.device.SetDown()
	}
	return nil
}

func (conn *zConnection) open() error {
	if conn.device == nil {
		return fmt.Errorf("require an interface name. Have you called Connect()")
	}
	if conn.counter == 0 {
		var err error
		conn.conn, err = socketcan.DialContext(context.Background(), "can", conn.interfaceName)
		if err != nil {
			fmt.Println(err)
			return err
		}
		conn.counter = 1
	} else {
		conn.counter += 1
	}
	return nil
}

func (conn *zConnection) close() {
	if conn.counter == 0 {
		return
	}
	conn.counter -= 1
	if conn.counter == 0 {
		conn.conn.Close()
		conn.conn = nil
	}
}

func (conn zConnection) getReceiver() *socketcan.Receiver {
	return socketcan.NewReceiver(conn.conn)
}

func (conn zConnection) getTransmitter() *socketcan.Transmitter {
	return socketcan.NewTransmitter(conn.conn)
}
