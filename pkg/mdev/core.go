package mdev

import (
	"fmt"
	"log"

	"github.com/goburrow/modbus"
)

const ModbusBool string = "bool"
const ModbusInt16 string = "s16"
const ModbusUint16 string = "u16"
const ModbusUint32 string = "u32"
const ModbusInt32 string = "s32"
const ModbusIEEE32 string = "ieee32"

const ModbusCoil int = 1
const ModbusInput int = 2
const ModbusHolding int = 3

type ModbusDevice struct {
	Name      string
	USBDevice string
	SlaveID   byte

	registers []*register
	calls     []*registerCall

	handler *modbus.RTUClientHandler
	stopper chan bool
}

func NewModbusDeviceLocal(name string, usbdev string, id byte) *ModbusDevice {
	dev := ModbusDevice{Name: name, USBDevice: usbdev, SlaveID: id}
	dev.handler = modbus.NewRTUClientHandler(usbdev)
	dev.handler.SlaveId = id
	return &dev
}

func (md *ModbusDevice) SetSerial(spd int) {
	md.handler.BaudRate = spd
}

func (md *ModbusDevice) AddRegister(desc, units string, regno uint16, format string, factor uint16, typ int, offset int) {
	reg := newRegister(desc, units, regno, format, factor, typ, offset)
	md.registers = append(md.registers, reg)

	var found bool
	for _, call := range md.calls {
		if call.qty == 125 {
			continue
		}
		if reg.typ != call.typ {
			continue
		}
		if (call.start <= regno && call.end >= regno) || call.end == regno {
			call.addRegister(reg)
			found = true
			break
		}
	}
	if !found {
		rcall := registerCall{reg.typ, reg.register, reg.endRegister(), reg.nRegisters, []*register{reg}}
		md.calls = append(md.calls, &rcall)
	}
}

/*

func (md *ModbusDevice) ShowCalls() {
	for n, call := range md.calls {
		fmt.Printf("%d: Call: %d - %d, %d\n", n, call.start, call.end, call.qty)
	}
}

func (md *ModbusDevice) ShowData() {
	for n, reg := range md.registers {
		fmt.Printf("%d: Call: %s [%s] %v\n", n, reg.description, reg.tag, reg.rawValue)
	}
}
*/

func (md *ModbusDevice) ReadOnce() error {
	err := md.handler.Connect()
	if err != nil {
		log.Println(err)
		return err
	}
	defer md.handler.Close()

	client := modbus.NewClient(md.handler)

	readCompleted := 0
	for n, call := range md.calls {
		var data []byte
		var err error
		switch call.typ {
		case ModbusHolding:
			data, err = client.ReadHoldingRegisters(call.start, call.qty)
		case ModbusInput:
			data, err = client.ReadInputRegisters(call.start, call.qty)
		}

		if err != nil {
			log.Printf("unable to get data for call #%d: %s", n, err)
			continue
		}
		readCompleted++
		call.processData(data)
	}
	if readCompleted == 0 {
		log.Printf("Unable to read any data from %s", md.USBDevice)
		return fmt.Errorf("failed to read data")
	}
	return nil
}

func (md *ModbusDevice) JsonResponse() map[string]interface{} {
	rv := make(map[string]interface{})
	for _, reg := range md.registers {
		if len(reg.rawValue) == 0 {
			continue
		}
		v := reg.getValue()
		if v == nil {
			log.Printf("unable to get data for register %s [%s]", reg.description, reg.tag)
			continue
		}
		rv[reg.tag] = reg.getValue()
	}
	return rv
}
