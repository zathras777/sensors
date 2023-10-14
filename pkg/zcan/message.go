package zcan

import (
	"encoding/hex"
	"fmt"
	"strings"

	"go.einride.tech/can"
)

type MsgType uint8

const (
	PDOData MsgType = iota
	HeartBeat
	RMI
)

type ZehnderCanMessage struct {
	CanId   uint32
	NodeId  uint32
	Data    [8]byte
	Length  uint8
	Remote  bool
	msgType MsgType
}

func MessageFromFrame(frame can.Frame) ZehnderCanMessage {
	msg := ZehnderCanMessage{CanId: frame.ID, Length: frame.Length, Remote: frame.IsRemote}
	msg.NodeId = frame.ID & 0x3f
	if frame.ID>>24 == 0 {
		msg.msgType = PDOData
	} else if frame.ID>>24&0x1F == 0x1F {
		msg.msgType = RMI
	} else {
		msg.msgType = HeartBeat
	}

	var n uint8
	for n = 0; n < frame.Length; n++ {
		msg.Data[n] = frame.Data[n]
	}
	return msg
}

func (msg ZehnderCanMessage) String() string {
	var id string
	id = fmt.Sprintf("0x%08X: NodeID: 0x%02X ", msg.CanId, msg.NodeId)
	switch msg.msgType {
	case PDOData:
		id += "PDOData "
	case RMI:
		id += "RMI"
		if msg.Remote {
			id += " Request"
		}
	case HeartBeat:
		id += "HeartBeat "
	}
	id += fmt.Sprintf(" %d bytes ", msg.Length)
	if msg.Length > 0 {
		id += " 0x" + strings.ToUpper(hex.EncodeToString(msg.Data[:msg.Length]))
	}
	return id
}
