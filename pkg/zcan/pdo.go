package zcan

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	"go.einride.tech/can"
)

func slugify(orig string) string {
	slug := strings.ToLower(orig)
	slug = strings.ReplaceAll(slug, " ", "-")
	return slug
}

func (dev *ZehnderDevice) processPDOFrame() {
loop:
	for {
		select {
		case frame := <-dev.pdoQ:
			msg := pdoFromFrame(frame)
			if msg.pdoId == 0 {
				log.Println("Ignoring PDO with an ID of 0")
				continue
			}
			pv, ck := dev.pdoData[int(msg.pdoId)]
			if !ck {
				sensor := findSensor(int(msg.pdoId), msg.length)
				pv = &PDOValue{sensor, nil}
				dev.pdoData[int(msg.pdoId)] = pv
			}
			pv.Value = msg.data[:msg.length]
		case <-dev.stopSignal:
			break loop
		}
	}
}

type pdoMessage struct {
	nodeId uint32
	pdoId  uint32
	length int
	data   []byte
}

func pdoFromFrame(frame can.Frame) pdoMessage {
	return pdoMessage{
		nodeId: frame.ID & 0x3F,
		pdoId:  (frame.ID >> 14) & 0x7FF,
		length: int(frame.Length),
		data:   frame.Data[:frame.Length],
	}
}

func (dev *ZehnderDevice) RequestPDO(prod byte, pdo uint16, interval byte) {
	canid := uint32(pdo&0x7ff)<<14 + uint32(0x40+prod)
	frame := can.Frame{ID: canid, IsExtended: true, IsRemote: true}
	copy(frame.Data[:], []byte{interval})
	frame.Length = 1
	dev.txQ <- frame
}

func (dev *ZehnderDevice) RequestPDOBySlug(prod byte, pdoSlug string, interval byte) error {
	var pdo uint16 = 0
	for id, poss := range sensorData {
		if poss.slug == strings.ToLower(pdoSlug) {
			pdo = uint16(id)
			break
		}
	}
	if pdo == 0 {
		return fmt.Errorf("no matching PDO found for '%s'", pdoSlug)
	}
	canid := uint32(pdo&0x7ff)<<14 + uint32(0x40+prod)
	frame := can.Frame{ID: canid, IsExtended: true, IsRemote: true}
	copy(frame.Data[:], []byte{interval})
	frame.Length = 1
	dev.txQ <- frame
	return nil
}

func (pdo pdoMessage) String() string {
	return fmt.Sprintf("Node ID: %d, PDO ID: %d  => 0x%s",
		pdo.nodeId,
		pdo.pdoId,
		strings.ToUpper(hex.EncodeToString(pdo.data[:pdo.length])))
}

const (
	UNIT_WATT    = "W"
	UNIT_KWH     = "kWh"
	UNIT_CELCIUS = "°C"
	UNIT_PERCENT = "%"
	UNIT_RPM     = "rpm"
	UNIT_M3H     = "m³/h"
	UNIT_SECONDS = "seconds"
	UNIT_UNKNOWN = "unknown"
	UNIT_DAYS    = "Days"
)

type ZehnderType int

const (
	CN_BOOL   ZehnderType = iota // 00 (false), 01 (true)
	CN_UINT8                     // 00 (0) until ff (255)
	CN_UINT16                    // 3412 = 1234
	CN_UINT32                    // 7856 3412 = 12345678
	CN_INT8
	CN_INT16 //3412 = 1234
	CN_INT64
	CN_STRING
	CN_TIME
	CN_VERSION
)

type PDOSensor struct {
	Name          string
	slug          string
	Units         string
	DataType      ZehnderType
	DecimalPlaces int
}

type PDOValue struct {
	Sensor PDOSensor
	Value  []byte
}

var sensorData = map[int]PDOSensor{
	49:  {"Operating Mode", "operating_mode", UNIT_UNKNOWN, CN_INT8, 0},
	65:  {"Fan Speed Setting", "fan_speed_setting", UNIT_UNKNOWN, CN_INT8, 0},
	81:  {"Boost Period Remaining", "boost_period_remaining", UNIT_SECONDS, CN_UINT32, 0},
	117: {"Exhaust Fan Duty", "exhaust_fan_duty", UNIT_PERCENT, CN_UINT8, 0},
	118: {"Supply Fan Duty", "supply_fan_duty", UNIT_PERCENT, CN_UINT8, 0},
	119: {"Exhaust Fan Flow", "exhaust_fan_flow", UNIT_M3H, CN_UINT16, 0},
	120: {"Supply Fan Flow", "supply_fan_flow", UNIT_M3H, CN_UINT16, 0},
	121: {"Exhaust Fan Speed", "exhaust_fan_speed", UNIT_RPM, CN_UINT16, 0},
	122: {"Supply Fan Speed", "supply_fan_speed", UNIT_RPM, CN_UINT16, 0},
	128: {"Power Consumption", "power_consumption", UNIT_WATT, CN_UINT16, 0},
	130: {"Power Consumption Total", "power_consumption_total", UNIT_KWH, CN_UINT16, 0},
	145: {"Preheater Power Consumption Total", "prehater_power_consumption_total", UNIT_KWH, CN_UINT16, 0},
	146: {"Preheater Power Consumption", "preheater_power_consumption", UNIT_WATT, CN_UINT16, 0},
	192: {"Filter Replacement Days", "filter_replacement_days", UNIT_DAYS, CN_UINT16, 0},
	209: {"RMOT", "rmot", UNIT_CELCIUS, CN_UINT16, 1},
	213: {"Avoided Heating Actual", "avoided_heating_actual", UNIT_WATT, CN_UINT16, 2},
	214: {"Avoided Heating YTD", "avoided_heating_ytd", UNIT_KWH, CN_UINT16, 0},
	220: {"Preheated Air Temperature (pre Heating)", "preheated_air_temperature_(pre_heating)", UNIT_CELCIUS, CN_UINT16, 1},
	221: {"Preheated Air Temperature (post Heating)", "preheated_air_temperature_(post_heating)", UNIT_CELCIUS, CN_UINT16, 1},
	227: {"Bypass State", "bypass_state", UNIT_PERCENT, CN_UINT8, 0},
	274: {"Extract Air Temperature", "extract_air_temperature", UNIT_CELCIUS, CN_UINT16, 1},
	275: {"Exhaust Air Temperature", "exhaust_air_temperature", UNIT_CELCIUS, CN_UINT16, 1},
	276: {"Outdoor Air Temperature", "outdoor_air_temperature", UNIT_CELCIUS, CN_UINT16, 1},
	277: {"Preheated Outside Air Temperature", "preheated_outside_air_temperature", UNIT_CELCIUS, CN_UINT16, 1},
	278: {"Supply Air Temperature", "supply_air_temperature", UNIT_CELCIUS, CN_UINT16, 1},
	290: {"Extract Humidity", "extract_humidity", UNIT_PERCENT, CN_UINT8, 0},
	291: {"Exhaust Humidity", "exhaust_humidity", UNIT_PERCENT, CN_UINT8, 0},
	292: {"Outdoor Humidity", "outdoor_humidity", UNIT_PERCENT, CN_UINT8, 0},
	293: {"Preheated Outdoor Humidity", "preheated_outdoor_humidity", UNIT_PERCENT, CN_UINT8, 0},
	294: {"Supply Air Humidity", "supply_air_humidity", UNIT_PERCENT, CN_INT8, 0},
}

func findSensor(pdo int, dataLen int) PDOSensor {
	sensor, ck := sensorData[pdo]
	if !ck {
		log.Printf("unknown sensor 0x%02x [%d] %d bytes of data", pdo, pdo, dataLen)
		sensorName := fmt.Sprintf("Unknown sensor %d", pdo)
		sensor = PDOSensor{sensorName, slugify(sensorName), UNIT_UNKNOWN, CN_UINT16, 0}
		if dataLen == 1 {
			sensor.DataType = CN_UINT8
		} else if dataLen == 4 {
			sensor.DataType = CN_UINT32
		}
		sensorData[pdo] = sensor
	}
	return sensor
}

func (pv PDOValue) GetData() interface{} {
	switch pv.Sensor.DataType {
	case CN_BOOL:
		return pv.Value[0] == 1
	case CN_STRING:
		rb := 0
		var c byte
		for rb, c = range pv.Value {
			if c == 0 {
				break
			}
		}
		return string(pv.Value[:rb+1])
	case CN_VERSION:
		return ZehnderVersionDecode(binary.LittleEndian.Uint32(pv.Value))
	case CN_UINT8, CN_UINT16, CN_UINT32:
		val := pv.Number()
		if pv.Sensor.DecimalPlaces > 0 {
			return float64(val) / (float64(pv.Sensor.DecimalPlaces) * 10)
		}
		return val
	case CN_INT8, CN_INT16, CN_INT64:
		val := pv.SignedNumber()
		if pv.Sensor.DecimalPlaces > 0 {
			return float64(val) / (float64(pv.Sensor.DecimalPlaces) * 10)
		}
		return val
	}
	return "Unknown"
}

func (pv PDOValue) String() string {
	s := fmt.Sprintf("%-45s0x%-8s", pv.Sensor.Name, strings.ToUpper(hex.EncodeToString(pv.Value)))
	if pv.IsFloat() {
		fmtS := fmt.Sprintf("  %%6.%df", pv.Sensor.DecimalPlaces)
		s += fmt.Sprintf(fmtS, pv.Float())
	} else if pv.IsSigned() {
		s += fmt.Sprintf("  %6d", pv.SignedNumber())
	} else {
		s += fmt.Sprintf("  %6d", pv.Number())
	}
	s += " " + pv.Sensor.Units
	return s
}

func (pv PDOValue) IsBool() bool   { return pv.Sensor.DataType == CN_BOOL }
func (pv PDOValue) IsString() bool { return pv.Sensor.DataType == CN_STRING }
func (pv PDOValue) IsFloat() bool  { return pv.Sensor.DecimalPlaces > 0 }
func (pv PDOValue) IsSigned() bool {
	return pv.Sensor.DataType == CN_INT8 || pv.Sensor.DataType == CN_INT16 || pv.Sensor.DataType == CN_INT64
}

func (pv PDOValue) Number() uint {
	if pv.Sensor.DataType == CN_INT16 || pv.Sensor.DataType == CN_INT8 || pv.Sensor.DataType == CN_INT64 {
		log.Println("attempt to get an unsigned number from a sensor with a signed data type?")
		return 0
	}
	switch pv.Sensor.DataType {
	case CN_UINT8:
		return uint(pv.Value[0])
	case CN_UINT16:
		return uint(binary.LittleEndian.Uint16(pv.Value))
	case CN_UINT32:
		return uint(binary.LittleEndian.Uint32(pv.Value))
	}
	return 0
}

func (pv PDOValue) SignedNumber() int {
	if pv.Sensor.DataType == CN_UINT16 || pv.Sensor.DataType == CN_UINT8 || pv.Sensor.DataType == CN_UINT32 {
		log.Println("attempt to get an signed number from a sensor with an unsigned data type?")
		return 0
	}
	switch pv.Sensor.DataType {
	case CN_INT8:
		return int(pv.Value[0])
	case CN_INT16:
		return int(binary.LittleEndian.Uint16(pv.Value))
	case CN_INT64:
		return int(binary.LittleEndian.Uint32(pv.Value))
	}
	return 0
}

func (pv PDOValue) Float() float64 {
	return float64(pv.Number()) / (float64(pv.Sensor.DecimalPlaces) * 10)
}
