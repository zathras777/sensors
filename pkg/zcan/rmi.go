package zcan

import (
	"encoding/binary"
	"fmt"
	"log"

	"go.einride.tech/can"
)

func (dev *ZehnderDevice) processRMIFrame() {
	var holder *ZehnderRMI
	dev.wg.Add(1)
	defer dev.wg.Done()

loop:
	for {
		select {
		case frame := <-dev.rmiQ:
			rmi := rmiFromFrame(frame)
			if rmi.DestId != dev.NodeID {
				if rmi.SourceId == dev.NodeID {
					continue
				}
				log.Printf("Received RMI but it's not for us...%02X vs wanted %02X\n", rmi.DestId, dev.NodeID)
				log.Printf("FRAME: %v\n", rmi)
				continue
			}
			if rmi.IsMulti {
				if holder != nil {
					holder.appendRMI(rmi)
				} else {
					holder = rmi
				}
				if holder.finalSeen {
					dev.doRMICallback(holder)
					holder = nil
				}
			} else {
				dev.doRMICallback(rmi)
			}
		case <-dev.stopSignal:
			break loop
		}
	}
}

func (dev *ZehnderDevice) doRMICallback(rmi *ZehnderRMI) {
	if dev.rmiCbFn != nil {
		dev.rmiCbFn(rmi)
		dev.rmiCbFn = nil
	} else if dev.defaultRMICbFn != nil {
		dev.defaultRMICbFn(rmi)
	} else {
		log.Println("RMI message received, but no callback was set?")
	}

	if dev.hasNetwork() {
		dev.rmiCTS <- true
	}
}

type ZehnderTypeFlag byte

const (
	ZehnderRMITypeNoValue     ZehnderTypeFlag = 0x00
	ZehnderRMITypeActualValue ZehnderTypeFlag = 0x10
	ZehnderRMITypeRange       ZehnderTypeFlag = 0x20
	ZehnderRMITypeStepSize    ZehnderTypeFlag = 0x40
)

type ZehnderRMI struct {
	SourceId   byte
	DestId     byte
	Sequence   byte
	Counter    byte
	IsMulti    bool
	IsRequest  bool
	IsError    bool
	Data       []byte
	DataLength int

	msgNo      byte
	finalSeen  bool
	callbackFn func(*ZehnderRMI)
	readPos    int
}

type ZehnderDestination struct {
	DestNodeId byte
	Unit       byte
	SubUnit    byte
}

func NewZehnderDestination(node byte, unit byte, subunit byte) ZehnderDestination {
	return ZehnderDestination{node, unit, subunit}
}

func (zr ZehnderDestination) GetOne(dev *ZehnderDevice, prop byte, flags ZehnderTypeFlag, cbFn func(*ZehnderRMI)) {
	rmi := ZehnderRMI{SourceId: dev.NodeID, DestId: zr.DestNodeId, IsRequest: true, Sequence: dev.rmiSequence}
	rmi.Data = []byte{0x01, zr.Unit, zr.SubUnit, byte(flags), prop}
	rmi.DataLength = 5
	rmi.callbackFn = cbFn
	dev.rmiSequence = (dev.rmiSequence + 1) & 0x03
	dev.rmiRequestQ <- &rmi
}

func (zr ZehnderDestination) GetMultiple(dev *ZehnderDevice, props []byte, flags ZehnderTypeFlag, cbFn func(*ZehnderRMI)) {
	rmi := ZehnderRMI{SourceId: dev.NodeID, DestId: zr.DestNodeId, IsRequest: true, Sequence: dev.rmiSequence}
	or_type := byte(flags) | byte(len(props))
	rmi.Data = append([]byte{0x02, zr.Unit, zr.SubUnit, 1, or_type}, props...)
	rmi.DataLength = len(rmi.Data)
	if rmi.DataLength > 8 {
		rmi.IsMulti = true
	}
	rmi.callbackFn = cbFn
	dev.rmiSequence = (dev.rmiSequence + 1) & 0x03
	dev.rmiRequestQ <- &rmi
}

func (zr ZehnderDestination) SetOne(dev *ZehnderDevice, prop byte, value []byte) {
	// Untested
	rmi := ZehnderRMI{SourceId: dev.NodeID, DestId: zr.DestNodeId, IsRequest: true, Sequence: dev.rmiSequence}
	rmi.Data = append([]byte{0x03, zr.Unit, zr.SubUnit, prop}, value...)
	dev.rmiSequence = (dev.rmiSequence + 1) & 0x03
	dev.rmiRequestQ <- &rmi
}

func rmiFromFrame(frame can.Frame) *ZehnderRMI {
	rmi := ZehnderRMI{SourceId: byte(frame.ID & 0x3F)}
	rmi.DestId = byte(frame.ID>>6) & 0x3F
	rmi.Counter = byte(frame.ID>>12) & 0x03
	rmi.Sequence = byte(frame.ID>>17) & 0x03
	rmi.IsMulti = frame.ID&(1<<14) == 1<<14
	rmi.IsError = frame.ID&(1<<15) == 1<<15
	rmi.IsRequest = frame.ID&(1<<16) == 1<<16
	rmi.Data = frame.Data[:frame.Length]
	rmi.DataLength = int(frame.Length)

	if !rmi.IsMulti {
		rmi.finalSeen = true
	} else {
		rmi.msgNo = rmi.Data[0]
		rmi.DataLength -= 1
		rmi.Data = rmi.Data[1:]
	}
	return &rmi
}

func (zrmi *ZehnderRMI) appendRMI(xtra *ZehnderRMI) {
	zrmi.msgNo = xtra.msgNo
	if zrmi.msgNo&0x80 == 0x80 {
		zrmi.finalSeen = true
		zrmi.msgNo &= 0x7F
	}
	zrmi.Data = append(zrmi.Data, xtra.Data...)
	zrmi.DataLength += xtra.DataLength
}

func (zrmi ZehnderRMI) MakeCANId() uint32 {
	can_id := uint32(0x1F000000) + uint32(zrmi.SourceId)
	can_id += uint32(zrmi.DestId) << 6
	can_id += uint32(zrmi.Counter&0x03) << 12
	if zrmi.IsMulti {
		can_id += (1 << 14)
	}
	if zrmi.IsError {
		can_id += (1 << 15)
	}
	if zrmi.IsRequest {
		can_id += (1 << 16)
	}
	can_id += uint32(zrmi.Sequence&0x03) << 17
	return can_id
}

func (zrmi *ZehnderRMI) send(dev *ZehnderDevice) error {
	dev.rmiCbFn = zrmi.callbackFn
	frame := can.Frame{ID: zrmi.MakeCANId(), IsExtended: true}
	copy(frame.Data[:], zrmi.Data[:])
	frame.Length = uint8(zrmi.DataLength)
	dev.txQ <- frame
	return nil
}

func (zrmi *ZehnderRMI) GetData(typ ZehnderType) (rv any, err error) {
	if zrmi.readPos >= zrmi.DataLength {
		err = fmt.Errorf("unable to extract any more data from the RMI data")
		return
	}
	data := zrmi.Data[zrmi.readPos:]
	switch typ {
	case CN_BOOL:
		rv = data[0] == 1
		zrmi.readPos++
	case CN_STRING:
		rb := 0
		var c byte
		for rb, c = range data[:zrmi.DataLength] {
			if c == 0 {
				break
			}
		}
		rv = string(data[:rb])
		zrmi.readPos += rb + 1
	case CN_VERSION:
		vers := ZehnderVersionDecode(binary.LittleEndian.Uint32(data))
		rv = fmt.Sprintf("%d.%d", vers[0], vers[1])
		zrmi.readPos += 4
	case CN_UINT8:
		rv = uint(data[0])
		zrmi.readPos++
	case CN_UINT16:
		rv = uint(binary.LittleEndian.Uint16(data))
		zrmi.readPos += 2
	case CN_UINT32:
		rv = uint(binary.LittleEndian.Uint32(data))
		zrmi.readPos += 4
	case CN_INT8:
		rv = int(data[0])
		zrmi.readPos++
	case CN_INT16:
		rv = int(binary.LittleEndian.Uint16(data))
		zrmi.readPos += 2
	case CN_INT64:
		rv = int(binary.LittleEndian.Uint32(data))
		zrmi.readPos += 8
	}
	return
}

func (dev *ZehnderDevice) processRMIQueue() {
	dev.wg.Add(1)
loop:
	for {
		select {
		case <-dev.rmiCTS:
			select {
			case rmi := <-dev.rmiRequestQ:
				rmi.send(dev)
			case <-dev.stopSignal:
				break loop
			}
		case <-dev.stopSignal:
			break loop
		}
	}
	dev.wg.Done()
}
