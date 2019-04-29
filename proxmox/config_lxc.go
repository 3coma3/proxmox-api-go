package proxmox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	// "math/rand"
	// "net"
	"net/url"
	"regexp"
	// "strconv"
	"strings"
	"time"
)

type (
	LxcDevices     map[int]map[string]interface{}
	LxcDevice      map[string]interface{}
	LxcDeviceParam []string
)

// ConfigLxc - Proxmox API LXC options
type ConfigLxc struct {
	Arch        string `json:"arch"`
	Cmode       string `json:"cmode"`
	Console     bool   `json:"console"`
	Cores       int    `json:"cores"`
	Cpulimit    int    `json:"cpulimit"`
	Cpuunits    int    `json:"cpuunits"`
	Description string `json:"description"`
	Digest      string `json:"digest"`
	Features    string `json:"features"`
	Hookscript  string `json:"hookscript"`
	Hostname    string `json:"hostname"`
	// Lock		enum
	Memory int `json:"memory"`
	// Mp[n]	string `json:"volumes"`
	Nameserver   string     `json:"nameserver"`
	Net          LxcDevices `json:"net"`
	Onboot       bool       `json:"onboot"`
	Ostype       string     `json:"ostype"`
	Protection   bool       `json:"protection"`
	Rootfs       string     `json:"rootfs"`
	Searchdomain string     `json:"searchdomain"`
	// Startup  string `json:"startup"`
	Swap         int    `json:"swap"`
	Template     bool   `json:"template"`
	Tty          int    `json:"tty"`
	Unprivileged bool   `json:"unprivileged"`
	Sshkeys      string `json:"sshkeys"`
}

// CreateVm - Tell Proxmox API to make the VM
func (config ConfigLxc) CreateVm(vmr *VmRef, client *Client) (err error) {
	vmr.SetVmType("lxc")

	params := map[string]interface{}{
		"vmid":         vmr.vmId,
		"arch":         config.Arch,
		"cmode":        config.Cmode,
		"console":      config.Console,
		"cores":        config.Cores,
		"cpulimit":     config.Cpulimit,
		"cpuunits":     config.Cpuunits,
		"description":  config.Description,
		"digest":       config.Digest,
		"features":     config.Features,
		"hookscript":   config.Hookscript,
		"hostname":     config.Hostname,
		"memory":       config.Memory,
		"nameserver":   config.Nameserver,
		"onboot":       config.Onboot,
		"ostype":       config.Ostype,
		"protection":   config.Protection,
		"rootfs":       config.Rootfs,
		"Swap":         config.Swap,
		"searchdomain": config.Searchdomain,
		"template":     config.Template,
		"tty":          config.Tty,
		"unprivileged": config.Unprivileged,
		"sshkeys":      config.Sshkeys,
	}

	// Create disks config.
	// config.CreateLxcDisksParams(vmr.vmId, params, false)

	// Create networks config.
	// config.CreateLxcNetworksParams(vmr.vmId, params)

	exitStatus, err := client.CreateLxcVm(vmr.node, params)
	if err != nil {
		return fmt.Errorf("Error creating VM: %v, error status: %s (params: %v)", err, exitStatus, params)
	}
	return
}

/*

CloneVm
Example: Request

nodes/proxmox1-xx/lxc/1012/clone

newid:145
name:tf-clone1
target:proxmox1-xx
full:1
storage:xxx

*/
func (config ConfigLxc) CloneVm(sourceVmr *VmRef, vmr *VmRef, client *Client) (err error) {
	vmr.SetVmType("lxc")

	params := map[string]interface{}{
		"newid":    vmr.vmId,
		"target":   vmr.node,
		"hostname": config.Hostname,
	}
	_, err = client.CloneLxcVm(sourceVmr, params)
	if err != nil {
		return
	}
	return config.UpdateConfig(vmr, client)
}

func (config ConfigLxc) UpdateConfig(vmr *VmRef, client *Client) (err error) {
	configParams := map[string]interface{}{
		"hostname":    config.Hostname,
		"description": config.Description,
		"onboot":      config.Onboot,
		"cores":       config.Cores,
		"memory":      config.Memory,
	}

	// Create disks config.
	// config.CreateLxcDisksParams(vmr.vmId, configParams, true)

	// Create networks config.
	// config.CreateLxcNetworksParams(vmr.vmId, configParams)

	if config.Searchdomain != "" {
		configParams["searchdomain"] = config.Searchdomain
	}
	if config.Nameserver != "" {
		configParams["nameserver"] = config.Nameserver
	}
	if config.Sshkeys != "" {
		sshkeyEnc := url.PathEscape(config.Sshkeys + "\n")
		sshkeyEnc = strings.Replace(sshkeyEnc, "+", "%2B", -1)
		sshkeyEnc = strings.Replace(sshkeyEnc, "@", "%40", -1)
		sshkeyEnc = strings.Replace(sshkeyEnc, "=", "%3D", -1)
		configParams["sshkeys"] = sshkeyEnc
	}
	_, err = client.SetVmConfig(vmr, configParams)
	return err
}

// this factory returns a new struct with the members set to the PVEAPI defaults
func NewConfigLxc() *ConfigLxc {
	return &ConfigLxc{
		Arch:         "arch64",
		Cmode:        "tty",
		Console:      true,
		Cores:        1,
		Cpulimit:     0,
		Cpuunits:     1024,
		Description:  "",
		Digest:       "",
		Features:     "",
		Hookscript:   "",
		Hostname:     "",
		Memory:       512,
		Nameserver:   "",
		Net:          LxcDevices{},
		Onboot:       false,
		Ostype:       "unmanaged",
		Protection:   false,
		Rootfs:       "",
		Searchdomain: "",
		Swap:         512,
		Template:     false,
		Tty:          2,
		Unprivileged: false,
	}
}

func NewConfigLxcFromJson(io io.Reader) (config *ConfigLxc, err error) {
	config = NewConfigLxc()
	err = json.NewDecoder(io).Decode(config)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return
}

func NewConfigLxcFromApi(vmr *VmRef, client *Client) (config *ConfigLxc, err error) {
	config = NewConfigLxc()

	var vmConfig map[string]interface{}
	for ii := 0; ii < 3; ii++ {
		vmConfig, err = client.GetVmConfig(vmr)
		if err != nil {
			log.Fatal(err)
			return nil, err
		}
		// this can happen:
		// {"data":{"lock":"clone","digest":"eb54fb9d9f120ba0c3bdf694f73b10002c375c38","description":" qmclone temporary file\n"}})
		if vmConfig["lock"] == nil {
			break
		} else {
			time.Sleep(8 * time.Second)
		}
	}

	if vmConfig["lock"] != nil {
		return nil, errors.New("vm locked, could not obtain config")
	}

	if _, isSet := vmConfig["arch"]; isSet {
		config.Arch = vmConfig["arch"].(string)
	}
	if _, isSet := vmConfig["cmode"]; isSet {
		config.Cmode = vmConfig["cmode"].(string)
	}
	if _, isSet := vmConfig["console"]; isSet {
		config.Console = Itob(int(vmConfig["console"].(float64)))
	}
	if _, isSet := vmConfig["cores"]; isSet {
		config.Cores = int(vmConfig["cores"].(float64))
	}
	if _, isSet := vmConfig["cpulimit"]; isSet {
		config.Cpulimit = int(vmConfig["cpulimit"].(float64))
	}
	if _, isSet := vmConfig["cpuunits"]; isSet {
		config.Cpuunits = int(vmConfig["cpuunits"].(float64))
	}
	if _, isSet := vmConfig["description"]; isSet {
		config.Description = vmConfig["description"].(string)
	}
	if _, isSet := vmConfig["digest"]; isSet {
		config.Digest = vmConfig["digest"].(string)
	}
	if _, isSet := vmConfig["features"]; isSet {
		config.Features = vmConfig["features"].(string)
	}
	if _, isSet := vmConfig["hookscript"]; isSet {
		config.Hookscript = vmConfig["hookscript"].(string)
	}
	if _, isSet := vmConfig["hostname"]; isSet {
		config.Hostname = vmConfig["hostname"].(string)
	}
	if _, isSet := vmConfig["memory"]; isSet {
		config.Memory = int(vmConfig["memory"].(float64))
	}
	if _, isSet := vmConfig["nameserver"]; isSet {
		config.Nameserver = vmConfig["nameserver"].(string)
	}
	if _, isSet := vmConfig["onboot"]; isSet {
		config.Onboot = Itob(int(vmConfig["onboot"].(float64)))
	}
	if _, isSet := vmConfig["ostype"]; isSet {
		config.Ostype = vmConfig["ostype"].(string)
	}
	if _, isSet := vmConfig["protection"]; isSet {
		config.Protection = Itob(int(vmConfig["protection"].(float64)))
	}
	if _, isSet := vmConfig["rootfs"]; isSet {
		config.Rootfs = vmConfig["rootfs"].(string)
	}
	if _, isSet := vmConfig["searchdomain"]; isSet {
		config.Searchdomain = vmConfig["searchdomain"].(string)
	}
	if _, isSet := vmConfig["swap"]; isSet {
		config.Swap = int(vmConfig["swap"].(float64))
	}
	if _, isSet := vmConfig["template"]; isSet {
		config.Template = Itob(int(vmConfig["template"].(float64)))
	}
	if _, isSet := vmConfig["tty"]; isSet {
		config.Tty = int(vmConfig["tty"].(float64))
	}
	if _, isSet := vmConfig["unprivileged"]; isSet {
		config.Unprivileged = Itob(int(vmConfig["unprivileged"].(float64)))
	}

	if _, isSet := vmConfig["sshkeys"]; isSet {
		config.Sshkeys, _ = url.PathUnescape(vmConfig["sshkeys"].(string))
	}

	// Add networks.
	nicNameRe := regexp.MustCompile(`net\d+`)
	nicNames := []string{}

	for k, _ := range vmConfig {
		if nicName := nicNameRe.FindStringSubmatch(k); len(nicName) > 0 {
			nicNames = append(nicNames, nicName[0])
		}
	}

	for _, nicName := range nicNames {
		nicConfStr := vmConfig[nicName]
		nicConfList := strings.Split(nicConfStr.(string), ",")

		//
		// id := rxDeviceID.FindStringSubmatch(nicName)
		// nicID, _ := strconv.Atoi(id[0])
		model, macaddr := ParseSubConf(nicConfList[0], "=")

		// Add model and MAC address.
		nicConfMap := LxcDevice{
			"model":   model,
			"macaddr": macaddr,
		}

		// Add rest of device config.
		nicConfMap.readDeviceConfig(nicConfList[1:])

		// And device config to networks.
		// if len(nicConfMap) > 0 {
		// 	config.LxcNetworks[nicID] = nicConfMap
		// }
	}

	return
}

// Create parameters for each Nic device.
// func (c ConfigLxc) CreateLxcNetworksParams(vmID int, params map[string]interface{}) error {

// 	// For backward compatibility.
// 	if len(c.LxcNetworks) == 0 && len(c.LxcNicModel) > 0 {
// 		deprecatedStyleMap := LxcDevice{
// 			"model":   c.LxcNicModel,
// 			"bridge":  c.LxcBrige,
// 			"macaddr": c.LxcMacAddr,
// 		}

// 		if c.LxcVlanTag > 0 {
// 			deprecatedStyleMap["tag"] = strconv.Itoa(c.LxcVlanTag)
// 		}

// 		c.LxcNetworks[0] = deprecatedStyleMap
// 	}

// 	// For new style with multi net device.
// 	for nicID, nicConfMap := range c.LxcNetworks {

// 		nicConfParam := LxcDeviceParam{}

// 		// Set Nic name.
// 		lxcNicName := "net" + strconv.Itoa(nicID)

// 		// Set Mac address.
// 		if nicConfMap["macaddr"] == nil || nicConfMap["macaddr"].(string) == "" {
// 			// Generate Mac based on VmID and NicID so it will be the same always.
// 			macaddr := make(net.HardwareAddr, 6)
// 			rand.Seed(time.Now().UnixNano())
// 			rand.Read(macaddr)
// 			macaddr[0] = (macaddr[0] | 2) & 0xfe // fix from github issue #18
// 			macAddrUppr := strings.ToUpper(fmt.Sprintf("%v", macaddr))
// 			// use model=mac format for older proxmox compatability
// 			macAddr := fmt.Sprintf("%v=%v", nicConfMap["model"], macAddrUppr)

// 			// Add Mac to source map so it will be returned. (useful for some use case like Terraform)
// 			nicConfMap["macaddr"] = macAddrUppr
// 			// and also add it to the parameters which will be sent to Proxmox API.
// 			nicConfParam = append(nicConfParam, macAddr)
// 		} else {
// 			macAddr := fmt.Sprintf("%v=%v", nicConfMap["model"], nicConfMap["macaddr"].(string))
// 			nicConfParam = append(nicConfParam, macAddr)
// 		}

// 		// Set bridge if not nat.
// 		if nicConfMap["bridge"].(string) != "nat" {
// 			bridge := fmt.Sprintf("bridge=%v", nicConfMap["bridge"])
// 			nicConfParam = append(nicConfParam, bridge)
// 		}

// 		// Keys that are not used as real/direct conf.
// 		ignoredKeys := []string{"id", "bridge", "macaddr", "model"}

// 		// Rest of config.
// 		nicConfParam = nicConfParam.createDeviceParam(nicConfMap, ignoredKeys)

// 		// Add nic to Lxc prams.
// 		params[lxcNicName] = strings.Join(nicConfParam, ",")
// 	}

// 	return nil
// }

// // Create parameters for each disk.
// func (c ConfigLxc) CreateLxcDisksParams(
// 	vmID int,
// 	params map[string]interface{},
// 	cloned bool,
// ) error {

// 	// For backward compatibility.
// 	if len(c.LxcDisks) == 0 && len(c.Storage) > 0 {

// 		dType := c.StorageType
// 		if dType == "" {
// 			if c.HasCloudInit() {
// 				dType = "scsi"
// 			} else {
// 				dType = "virtio"
// 			}
// 		}
// 		deprecatedStyleMap := LxcDevice{
// 			"type":         dType,
// 			"storage":      c.Storage,
// 			"size":         c.DiskSize,
// 			"storage_type": "lvm",  // default old style
// 			"cache":        "none", // default old value
// 		}

// 		c.LxcDisks[0] = deprecatedStyleMap
// 	}

// 	// For new style with multi disk device.
// 	for diskID, diskConfMap := range c.LxcDisks {

// 		// skip the first disk for clones (may not always be right, but a template probably has at least 1 disk)
// 		if diskID == 0 && cloned {
// 			continue
// 		}
// 		diskConfParam := LxcDeviceParam{
// 			"media=disk",
// 		}

// 		// Device name.
// 		deviceType := diskConfMap["type"].(string)
// 		lxcDiskName := deviceType + strconv.Itoa(diskID)

// 		// Set disk storage.
// 		// Disk size.
// 		diskSizeGB := fmt.Sprintf("size=%v", diskConfMap["size"])
// 		diskConfParam = append(diskConfParam, diskSizeGB)

// 		// Disk name.
// 		var diskFile string
// 		// Currently ZFS local, LVM, and Directory are considered.
// 		// Other formats are not verified, but could be added if they're needed.
// 		rxStorageTypes := `(zfspool|lvm)`
// 		storageType := diskConfMap["storage_type"].(string)
// 		if matched, _ := regexp.MatchString(rxStorageTypes, storageType); matched {
// 			diskFile = fmt.Sprintf("file=%v:vm-%v-disk-%v", diskConfMap["storage"], vmID, diskID)
// 		} else {
// 			diskFile = fmt.Sprintf("file=%v:%v/vm-%v-disk-%v.%v", diskConfMap["storage"], vmID, vmID, diskID, diskConfMap["format"])
// 		}
// 		diskConfParam = append(diskConfParam, diskFile)

// 		// Set cache if not none (default).
// 		if diskConfMap["cache"].(string) != "none" {
// 			diskCache := fmt.Sprintf("cache=%v", diskConfMap["cache"])
// 			diskConfParam = append(diskConfParam, diskCache)
// 		}

// 		// Keys that are not used as real/direct conf.
// 		ignoredKeys := []string{"id", "type", "storage", "storage_type", "size", "cache"}

// 		// Rest of config.
// 		diskConfParam = diskConfParam.createDeviceParam(diskConfMap, ignoredKeys)

// 		// Add back to Lxc prams.
// 		params[lxcDiskName] = strings.Join(diskConfParam, ",")
// 	}

// 	return nil
// }

// Create the parameters for each device that will be sent to Proxmox API.
func (p LxcDeviceParam) createDeviceParam(
	deviceConfMap LxcDevice,
	ignoredKeys []string,
) LxcDeviceParam {

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
func (confMap LxcDevice) readDeviceConfig(confList []string) error {
	// Add device config.
	for _, conf := range confList {
		key, value := ParseSubConf(conf, "=")
		confMap[key] = value
	}
	return nil
}

func (c ConfigLxc) String() string {
	jsConf, _ := json.Marshal(c)
	return string(jsConf)
}
