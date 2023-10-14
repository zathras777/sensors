package zcan

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

	"go.einride.tech/can"
)

func ZehnderVersionDecode(val uint32) []int {
	major := int(val>>30) & 3
	minor := int(val>>20) & 1023
	return []int{major, minor}
}

type ZehnderDevice struct {
	Name      string
	NodeID    byte
	Connected bool

	Model           string
	SerialNumber    string
	SoftwareVersion string

	connection zConnection

	wg             sync.WaitGroup
	routines       int
	stopSignal     chan bool
	frameQ         chan can.Frame
	pdoQ           chan can.Frame
	rmiQ           chan can.Frame
	txQ            chan can.Frame
	heartbeatQ     chan can.Frame
	rmiRequestQ    chan *ZehnderRMI
	info_syncer    chan bool
	rmiCTS         chan bool
	pdoData        map[int]*PDOValue
	rmiCbFn        func(*ZehnderRMI)
	defaultRMICbFn func(*ZehnderRMI)
	rmiSequence    byte
	captureFh      *os.File
	doCapture      bool
}

func NewZehnderDevice(id byte) *ZehnderDevice {
	return &ZehnderDevice{NodeID: id, pdoData: make(map[int]*PDOValue), Name: "Zehnder MVHR"}
}

func (dev *ZehnderDevice) SetDefaultRMICallback(fn func(*ZehnderRMI)) {
	dev.defaultRMICbFn = fn
}

func (dev *ZehnderDevice) Connect(interfaceName string) error {
	return dev.connection.open_device(interfaceName)
}

func (dev *ZehnderDevice) Start() error {
	dev.stopSignal = make(chan bool, 2)
	dev.frameQ = make(chan can.Frame)
	dev.pdoQ = make(chan can.Frame)
	dev.rmiQ = make(chan can.Frame)
	dev.txQ = make(chan can.Frame)
	dev.heartbeatQ = make(chan can.Frame)
	dev.rmiRequestQ = make(chan *ZehnderRMI)
	dev.rmiCTS = make(chan bool)

	go dev.processFrame()
	go dev.processPDOFrame()
	go dev.processRMIFrame()
	go dev.processRMIQueue()
	go dev.heartbeat()
	dev.routines = 5

	if dev.connection.device != nil {
		log.Println("Starting network services")
		// The receiver does not participate in the wait group, so
		// don't include in the numbers...
		go dev.receiver()
		go dev.transmitter()
		dev.rmiCTS <- true
		dev.routines = 6
	}

	return nil
}

func (dev *ZehnderDevice) hasNetwork() bool {
	return dev.connection.conn != nil
}

func (dev *ZehnderDevice) storeDeviceInfo(rmi *ZehnderRMI) {
	tmp, err := rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get device serial number: %s", err)
		dev.info_syncer <- false
		return
	}
	dev.SerialNumber = tmp.(string)
	tmp, err = rmi.GetData(CN_VERSION)
	if err != nil {
		log.Printf("unable to get software version from device: %s", err)
		dev.info_syncer <- false
		return
	}
	dev.SoftwareVersion = tmp.(string)
	tmp, err = rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get device model description: %s", err)
		dev.info_syncer <- false
		return
	}
	dev.Model = tmp.(string)
	dev.info_syncer <- true
}

func (dev *ZehnderDevice) getDeviceInfo(syncer chan bool) {
	dest := NewZehnderDestination(1, 1, 1)
	dest.GetMultiple(dev, []byte{4, 6, 8}, ZehnderRMITypeActualValue, dev.storeDeviceInfo)
	dev.info_syncer = syncer
}

func (dev *ZehnderDevice) Wait() {
	dev.wg.Wait()
}

func (dev *ZehnderDevice) Stop() {
	for n := 0; n < dev.routines; n++ {
		dev.stopSignal <- true
	}
}

func (dev *ZehnderDevice) CaptureAll(fn string) error {
	f, err := os.Create(fn)
	if err != nil {
		fmt.Println(err)
		return err
	}
	dev.captureFh = f
	dev.doCapture = true
	return nil
}

func (dev *ZehnderDevice) ProcessDumpFile(filename string) (err error) {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		fmt.Printf("File does not exist: %s. Error: %s\n", filename, err)
		return err
	}
	fmt.Printf("File: %s. Total size is %v bytes\n", filename, info.Size())
	if info.Size() == 0 {
		fmt.Println("File has 0 bytes. Nothing to do")
		return fmt.Errorf("file has zero size. Nothing to do")
	}

	readFile, err := os.Open(filename)

	if err != nil {
		fmt.Println(err)
		return err
	}
	fileScanner := bufio.NewScanner(readFile)

	fileScanner.Split(bufio.ScanLines)

	for fileScanner.Scan() {
		fmt.Print(".")
		frame := can.Frame{}
		frame.UnmarshalString(fileScanner.Text())
		dev.frameQ <- frame
	}
	fmt.Println()

	readFile.Close()
	return err
}

type pair struct {
	key   int
	value *PDOValue
}
type pairList []pair

func (p pairList) Len() int           { return len(p) }
func (p pairList) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p pairList) Less(i, j int) bool { return p[i].value.Sensor.Name < p[j].value.Sensor.Name }

func (dev *ZehnderDevice) JsonResponse() map[string]interface{} {
	dataMap := make(map[string]interface{})

	for _, v := range dev.pdoData {
		dataMap[v.Sensor.slug] = v.GetData()
	}
	return dataMap
}

func (dev *ZehnderDevice) JsonDeviceInfo() map[string]interface{} {
	if dev.SerialNumber == "" {
		syncer := make(chan bool)
		dev.getDeviceInfo(syncer)
		<-syncer
	}
	dataMap := make(map[string]interface{})
	dataMap["model"] = dev.Model
	dataMap["serial_number"] = dev.SerialNumber
	dataMap["software_version"] = dev.SoftwareVersion
	return dataMap
}

func (dev *ZehnderDevice) DumpPDO() {
	p := make(pairList, len(dev.pdoData))
	i := 0

	for k, v := range dev.pdoData {
		p[i] = pair{k, v}
		i++
	}

	sort.Sort(p)

	fmt.Println()
	fmt.Println("ID   Name                                         Raw Data     Value Units")
	fmt.Println("---- -------------------------------------------- ---------- ------- ---------")
	for _, k := range p {
		fmt.Printf("%3d  %s\n", k.key, k.value)
	}
	fmt.Println()
}
