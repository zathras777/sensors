package mdev

import (
	"encoding/binary"
	"math"
	"sort"
)

type register struct {
	description string
	tag         string
	register    uint16
	format      string
	factor      uint16
	typ         int
	offset      int

	nRegisters uint16
	nBytes     int

	rawValue []byte
}

type registerCall struct {
	typ       int
	start     uint16
	end       uint16
	qty       uint16
	registers []*register
}

func newRegister(desc, tag string, regno uint16, format string, factor uint16, typ int, offset int) *register {
	reg := register{description: desc, tag: tag, register: regno, format: format, factor: factor, typ: typ, offset: offset, nRegisters: 1, nBytes: 2}

	switch format {
	case ModbusBool:
		if reg.typ == ModbusCoil {
			reg.nBytes = 1
		}
	case ModbusUint32, ModbusInt32, ModbusIEEE32:
		reg.nRegisters = 2
		reg.nBytes = 4
	}
	return &reg
}

func (r register) endRegister() uint16 {
	return r.register + r.nRegisters
}

func (rc *registerCall) addRegister(reg *register) {
	rc.registers = append(rc.registers, reg)

	sort.Slice(rc.registers, func(i, j int) bool {
		return rc.registers[i].register < rc.registers[j].register
	})

	if rc.start > reg.register {
		rc.start = reg.register
	}
	if rc.end < reg.endRegister() {
		rc.end = reg.endRegister()
	}
	rc.qty = rc.end - rc.start
}

func (rc *registerCall) processData(data []byte) {
	pos := 0
	for _, reg := range rc.registers {
		reg.rawValue = data[pos : pos+reg.nBytes]
		pos += reg.nBytes
	}
}

func (r *register) getValue() interface{} {
	switch r.format {
	case ModbusBool:
		if r.typ == ModbusCoil {
			return r.rawValue[0] == 1
		}
		return r.rawValue[1] == 1
	case ModbusInt16:
		v := int16(binary.BigEndian.Uint16(r.rawValue))
		if r.offset != 0 {
			v += int16(r.offset) * int16(r.factor*10)
		}
		if r.factor != 0 {
			return float64(v) / float64(r.factor*10)
		}
		return v
	case ModbusUint16:
		v := binary.BigEndian.Uint16(r.rawValue)
		if r.offset != 0 {
			v += uint16(r.offset) * uint16(r.factor*10)
		}
		if r.factor != 0 {
			return float64(v) / float64(r.factor*10)
		}
		return v
	case ModbusInt32:
		v := int32(binary.BigEndian.Uint32(r.rawValue))
		if r.offset != 0 {
			v += int32(r.offset) * int32(r.factor*10)
		}
		if r.factor > 0 {
			return float64(v) / float64(r.factor*10)
		}
		return v
	case ModbusUint32:
		v := binary.BigEndian.Uint32(r.rawValue)
		if r.offset != 0 {
			v += uint32(r.offset) * uint32(r.factor*10)
		}
		if r.factor > 0 {
			return float64(v) / float64(r.factor*10)
		}
		return v
	case ModbusIEEE32:
		u := binary.BigEndian.Uint32(r.rawValue)
		sign := u >> 31
		exp := float64(u>>23&0xff) - 0x7f
		rem := uint64(u & 0x7fffff)
		var bottom uint64
		if exp != 0 {
			bottom = 0x800000
		} else {
			bottom = 0x400000
		}
		mant := float64(rem)/float64(bottom) + 1

		if sign == 0 {
			return mant * math.Exp2(exp)
		}
		return -1 * mant * math.Exp2(exp)
	}
	return nil
}
