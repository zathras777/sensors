package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"syscall"

	"github.com/zathras777/sensors/pkg/mdev"
	"github.com/zathras777/sensors/pkg/zcan"
)

var setupZcan []*zcan.ZehnderDevice
var setupModbus []*mdev.ModbusDevice

func main() {
	if err := processConfigurationFile("./config.yaml"); err != nil {
		log.Fatal(err)
	}

	for _, zcan := range cfg.Zcan {
		addZcan(zcan)
	}

	for _, node := range cfg.Modbus {
		addModbus(node)
	}

	if len(setupZcan)+len(setupModbus) == 0 {
		log.Fatal("Unable to configure any services. Nothing to do? Exiting")
	}

	go startHttpServer(cfg.Http.Address, cfg.Http.Port)

	sigs := make(chan os.Signal, 1)
	waiter := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		httpServer.Close()
		for _, zc := range setupZcan {
			zc.Stop()
		}
		for _, md := range setupModbus {
			md.Stop()
		}
		waiter <- true
	}()
	<-waiter
	log.Print("closing down")
}

func addZcan(node ZcanNode) error {
	zc := zcan.NewZehnderDevice(node.NodeId)
	if err := zc.Connect(node.Interface); err != nil {
		log.Printf("unable to connect to %s for zcan service %s: %s", node.Interface, node.Name, err)
		return err
	}

	if err := zc.Start(); err != nil {
		log.Printf("unable to start the zcan service %s: %s", zc.Name, err)
		return err
	}

	for _, pdo := range node.PDO.PDO {
		if err := zc.RequestPDOBySlug(node.PDO.Node, pdo.Slug, pdo.Interval); err != nil {
			log.Printf("unable to add PDO '%s': %s", pdo.Slug, err)
		}
	}

	AddEndpoint(JsonEndpoint{fmt.Sprintf("/%s", node.Name), zc.JsonResponse})
	AddEndpoint(JsonEndpoint{fmt.Sprintf("/%s/device-info", node.Name), zc.JsonDeviceInfo})
	log.Printf("zcan service %s setup OK", node.Name)
	setupZcan = append(setupZcan, zc)
	return nil
}

func addModbus(node ModbusNode) error {
	md := mdev.NewModbusDeviceLocal(node.Name, node.Device, node.SlaveId)
	if node.Baudrate > 0 {
		md.SetSerial(node.Baudrate)
	}
	sort.Slice(node.Registers.Holding, func(i, j int) bool {
		return node.Registers.Holding[i].Register < node.Registers.Holding[j].Register
	})

	for _, reg := range node.Registers.Holding {
		md.AddRegister(reg.Description, reg.Tag, reg.Register, reg.Typ, reg.Factor, mdev.ModbusHolding, reg.Offset)
	}

	sort.Slice(node.Registers.Input, func(i, j int) bool {
		return node.Registers.Input[i].Register < node.Registers.Input[j].Register
	})
	for _, reg := range node.Registers.Input {
		md.AddRegister(reg.Description, reg.Tag, reg.Register, reg.Typ, reg.Factor, mdev.ModbusInput, reg.Offset)
	}

	md.ReadOnce()
	md.Start(node.Interval)
	AddEndpoint(JsonEndpoint{fmt.Sprintf("/%s", node.Name), md.JsonResponse})
	log.Printf("modbus service %s setup OK", node.Name)
	setupModbus = append(setupModbus, md)
	return nil
}
