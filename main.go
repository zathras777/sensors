package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"

	"github.com/zathras777/sensors/pkg/max6675"
	"github.com/zathras777/sensors/pkg/mdev"
	"github.com/zathras777/sensors/pkg/zcan"
)

var setupZcan []*zcan.ZehnderDevice
var setupModbus []*mdev.ModbusDevice
var setupMax6675 []*max6675.Max6675Device

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

	for _, node := range cfg.Max6675 {
		addMax6675(node)
	}

	if len(setupZcan)+len(setupModbus)+len(setupMax6675) == 0 {
		log.Fatal("Unable to configure any services. Nothing to do? Exiting")
	}

	sigs := make(chan os.Signal, 1)
	failedHttp := make(chan bool, 1)
	waiter := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := startHttpServer(cfg.Http.Address, cfg.Http.Port); err != nil {
			failedHttp <- true
		}
	}()

	go func() {
		select {
		case <-sigs:
			break
		case <-failedHttp:
			log.Println("failed to start the HTTP server, exiting...")
		}
		httpServer.Close()
		for _, zc := range setupZcan {
			zc.Stop()
		}
		for _, md := range setupModbus {
			md.Stop()
		}
		for _, m6 := range setupMax6675 {
			m6.Stop()
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
		if pdo.Slug == "" {
			continue
		}
		if err := zc.RequestPDOBySlug(node.PDO.Node, pdo.Slug, pdo.Interval); err != nil {
			log.Printf("unable to add PDO '%s': %s", pdo.Slug, err)
		}
	}
	slug := endpointSlugify(node.Name)
	AddEndpoint(JsonEndpoint{slug, zc.JsonResponse})
	AddEndpoint(JsonEndpoint{fmt.Sprintf("%s/device-info", slug), zc.JsonDeviceInfo})
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
	slug := endpointSlugify(node.Name)
	AddEndpoint(JsonEndpoint{slug, md.JsonResponse})
	log.Printf("modbus service %s setup OK", node.Name)
	setupModbus = append(setupModbus, md)
	return nil
}

func addMax6675(node Max6675Node) error {
	m6 := max6675.NewMax6675(node.Name, node.Path, node.Interval)
	if err := m6.Start(); err != nil {
		log.Printf("unable to start MAX6675 service %s: %s", node.Name, err)
		return err
	}
	name := endpointSlugify(node.Name)
	AddEndpoint(JsonEndpoint{name, m6.JsonResponse})
	return nil
}

func endpointSlugify(orig string) string {
	slug := strings.ToLower(orig)
	slug = strings.ReplaceAll(slug, " ", "_")
	return "/" + slug
}
