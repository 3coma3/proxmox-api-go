package proxmox

import (
	"fmt"
	"strconv"
	"strings"
)

type (
	VmDevices     map[int]map[string]interface{}
	VmDevice      map[string]interface{}
	VmDeviceParam []string
)

func inArray(arr []string, str string) bool {
	for _, elem := range arr {
		if elem == str {
			return true
		}
	}

	return false
}

func Itob(i int) bool {
	if i == 1 {
		return true
	}
	return false
}

// ParseSubConf - Parse standard sub-conf strings `key=value`.
func ParseSubConf(
	element string,
	separator string,
) (key string, value interface{}) {
	if strings.Contains(element, separator) {
		conf := strings.Split(element, separator)
		key, value := conf[0], conf[1]
		var interValue interface{}

		// Make sure to add value in right type,
		// because all subconfig are returned as strings from Proxmox API.
		if iValue, err := strconv.ParseInt(value, 10, 64); err == nil {
			interValue = int(iValue)
		} else if bValue, err := strconv.ParseBool(value); err == nil {
			interValue = bValue
		} else {
			interValue = value
		}
		return key, interValue
	}
	return
}

// ParseConf - Parse standard device conf string `key1=val1,key2=val2`.
func ParseConf(
	kvString string,
	confSeparator string,
	subConfSeparator string,
) VmDevice {
	var confMap = QemuDevice{}
	confList := strings.Split(kvString, confSeparator)
	for _, item := range confList {
		key, value := ParseSubConf(item, subConfSeparator)
		confMap[key] = value
	}
	return confMap
}

// Create the parameters for each device that will be sent to Proxmox API.
func (p VmDeviceParam) createDeviceParam(
	deviceConfMap VmDevice,
	ignoredKeys []string,
) VmDeviceParam {

	for key, value := range deviceConfMap {
		if ignored := inArray(ignoredKeys, key); !ignored {
			var confValue interface{}
			if bValue, ok := value.(bool); ok && bValue {
				confValue = "1"
			} else if sValue, ok := value.(string); ok && len(sValue) > 0 {
				confValue = sValue
			} else if iValue, ok := value.(int); ok && iValue > 0 {
				confValue = iValue
			}
			if confValue != nil {
				deviceConf := fmt.Sprintf("%v=%v", key, confValue)
				p = append(p, deviceConf)
			}
		}
	}

	return p
}

// readDeviceConfig - get standard sub-conf strings where `key=value` and update conf map.
func (confMap VmDevice) readDeviceConfig(confList []string) error {
	// Add device config.
	for _, conf := range confList {
		key, value := ParseSubConf(conf, "=")
		confMap[key] = value
	}
	return nil
}
