package main

import (
	"log"
	"os"

	"gopkg.in/yaml.v2"
)

type ModbusRegister struct {
	Description string
	Tag         string
	Typ         string
	Register    uint16
	Factor      uint16
	Offset      int
}

type ModbusNode struct {
	Name      string
	SlaveId   byte
	Baudrate  int
	Device    string
	Interval  int
	Registers struct {
		Holding []ModbusRegister
		Input   []ModbusRegister
	}
}

type ZcanPDO struct {
	Slug     string
	Interval byte
}

type ZcanNode struct {
	Name      string
	Interface string
	NodeId    byte
	PDO       struct {
		Node byte
		PDO  []ZcanPDO
	}
}

type HttpNode struct {
	Address string
	Port    int
}

type ConfigFile struct {
	Http   HttpNode
	Zcan   []ZcanNode
	Modbus []ModbusNode
}

var cfg ConfigFile

func processConfigurationFile(fn string) error {
	dat, err := os.ReadFile(fn)
	if err != nil {
		log.Fatal(err)
	}
	return yaml.Unmarshal(dat, &cfg)
}
