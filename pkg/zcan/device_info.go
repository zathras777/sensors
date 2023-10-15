package zcan

import "log"

type ZehnderDeviceInfo struct {
	Model           string
	SerialNumber    string
	SoftwareVersion string
	ArticleNumber   string
	CountryCode     string
	DeviceName      string

	syncer chan bool
}

func NewZehnderDeviceInfo() *ZehnderDeviceInfo {
	return &ZehnderDeviceInfo{syncer: make(chan bool, 1)}
}

func (zdi *ZehnderDeviceInfo) storeDeviceInfo(rmi *ZehnderRMI) {
	tmp, err := rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get device serial number: %s", err)
		zdi.syncer <- false
		return
	}
	zdi.SerialNumber = tmp.(string)
	tmp, err = rmi.GetData(CN_VERSION)
	if err != nil {
		log.Printf("unable to get software version from device: %s", err)
		zdi.syncer <- false
		return
	}
	zdi.SoftwareVersion = tmp.(string)
	tmp, err = rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get device model description: %s", err)
		zdi.syncer <- false
		return
	}
	zdi.Model = tmp.(string)
	tmp, err = rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get article number : %s", err)
		zdi.syncer <- false
		return
	}
	zdi.ArticleNumber = tmp.(string)
	tmp, err = rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get country code: %s", err)
		zdi.syncer <- false
		return
	}
	zdi.CountryCode = tmp.(string)

	tmp, err = rmi.GetData(CN_STRING)
	if err != nil {
		log.Printf("unable to get device name: %s", err)
		zdi.syncer <- false
		return
	}
	zdi.DeviceName = tmp.(string)

	zdi.syncer <- true
}

func (zdi *ZehnderDeviceInfo) startUpdate(dev *ZehnderDevice) {
	log.Println("attempting to update device information")
	dest := NewZehnderDestination(1, 1, 1)
	dest.GetMultiple(dev, []byte{4, 6, 8, 0x0B, 0x0D, 0x14}, ZehnderRMITypeActualValue, zdi.storeDeviceInfo)
}

func (dev *ZehnderDevice) JsonDeviceInfo() map[string]interface{} {
	dataMap := make(map[string]interface{})
	if dev.DeviceInfo.DeviceName == "" {
		dev.DeviceInfo.startUpdate(dev)
		rv := <-dev.DeviceInfo.syncer
		if !rv {
			log.Printf("unable to get device information")
			return dataMap
		}
	}
	dataMap["model"] = dev.DeviceInfo.Model
	dataMap["serial_number"] = dev.DeviceInfo.SerialNumber
	dataMap["software_version"] = dev.DeviceInfo.SoftwareVersion
	dataMap["article_number"] = dev.DeviceInfo.ArticleNumber
	dataMap["country_code"] = dev.DeviceInfo.CountryCode
	dataMap["device_name"] = dev.DeviceInfo.DeviceName
	return dataMap
}
