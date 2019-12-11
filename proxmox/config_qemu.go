package proxmox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ConfigQemu - Proxmox API QEMU options
type ConfigQemu struct {
	Name        string    `json:"name"`
	Description string    `json:"desc"`
	Onboot      bool      `json:"onboot"`
	Agent       string    `json:"agent"`
	Memory      int       `json:"memory"`
	Ostype      string    `json:"ostype"`
	Cores       int       `json:"cores"`
	Sockets     int       `json:"sockets"`
	Iso         string    `json:"iso"`
	Disk        VmDevices `json:"disk"`
	Net         VmDevices `json:"net"`

	// cloud-init options
	CIuser     string `json:"ciuser"`
	CIpassword string `json:"cipassword"`

	Searchdomain string `json:"searchdomain"`
	Nameserver   string `json:"nameserver"`
	Sshkeys      string `json:"sshkeys"`

	// arrays are hard, support 2 interfaces for now
	Ipconfig0 string `json:"ipconfig0"`
	Ipconfig1 string `json:"ipconfig1"`

	Delete string `json:"delete"`
}

// CreateVm - Tell Proxmox API to make the VM
func (config ConfigQemu) CreateVm(vm *Vm) (err error) {
	if config.HasCloudInit() {
		return errors.New("Cloud-init parameters only supported on clones or updates")
	}
	vm.SetType("qemu")

	params := map[string]interface{}{
		"vmid":        vm.id,
		"name":        config.Name,
		"onboot":      config.Onboot,
		"agent":       config.Agent,
		"ide2":        config.Iso + ",media=cdrom",
		"ostype":      config.Ostype,
		"sockets":     config.Sockets,
		"cores":       config.Cores,
		"cpu":         "host",
		"memory":      config.Memory,
		"description": config.Description,
	}

	// Create disks config.
	config.CreateDisksParams(vm.id, params, false)

	// Create networks config.
	config.CreateNetParams(vm.id, params)

	if exitStatus, err := vm.Create(params); err != nil {
		err = fmt.Errorf("Error creating VM: %v, error status: %s (params: %v)", err, exitStatus, params)
	}

	return
}

// HasCloudInit - are there cloud-init options?
func (config ConfigQemu) HasCloudInit() bool {
	return config.CIuser != "" ||
		config.CIpassword != "" ||
		config.Searchdomain != "" ||
		config.Nameserver != "" ||
		config.Sshkeys != "" ||
		config.Ipconfig0 != "" ||
		config.Ipconfig1 != ""
}

func (config ConfigQemu) UpdateConfig(vm *Vm) (err error) {
	configParams := map[string]interface{}{
		"name":        config.Name,
		"description": config.Description,
		"onboot":      config.Onboot,
		"agent":       config.Agent,
		"sockets":     config.Sockets,
		"cores":       config.Cores,
		"memory":      config.Memory,
	}

	// Create disks config.
	config.CreateDisksParams(vm.id, configParams, true)

	// Create networks config.
	config.CreateNetParams(vm.id, configParams)

	// cloud-init options
	if config.CIuser != "" {
		configParams["ciuser"] = config.CIuser
	}
	if config.CIpassword != "" {
		configParams["cipassword"] = config.CIpassword
	}
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
	if config.Ipconfig0 != "" {
		configParams["ipconfig0"] = config.Ipconfig0
	}
	if config.Ipconfig1 != "" {
		configParams["ipconfig1"] = config.Ipconfig1
	}
	if config.Delete != "" {
		configParams["delete"] = config.Delete
	}

	_, err = vm.SetConfig(configParams)

	return
}

func NewConfigQemuFromJson(io io.Reader) (config *ConfigQemu, err error) {
	config = &ConfigQemu{}

	if err = json.NewDecoder(io).Decode(config); err == nil {
		log.Println(config)
	}

	return
}

var (
	rxIso      = regexp.MustCompile(`(.*?),media`)
	rxDeviceID = regexp.MustCompile(`\d+`)
	rxDiskName = regexp.MustCompile(`(virtio|scsi)\d+`)
	rxDiskType = regexp.MustCompile(`\D+`)
	rxNicName  = regexp.MustCompile(`net\d+`)
)

func NewConfigQemuFromApi(vm *Vm) (config *ConfigQemu, err error) {
	var vmConfig map[string]interface{}

	for ii := 0; ii < 3; ii++ {
		if vmConfig, err = vm.GetConfig(); err != nil {
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

	// vmConfig Sample: map[ cpu:host
	// net0:virtio=62:DF:XX:XX:XX:XX,bridge=vmbr0
	// ide2:local:iso/xxx-xx.iso,media=cdrom memory:2048
	// smbios1:uuid=8b3bf833-aad8-4545-xxx-xxxxxxx digest:aa6ce5xxxxx1b9ce33e4aaeff564d4 sockets:1
	// name:terraform-ubuntu1404-template bootdisk:virtio0
	// virtio0:ProxmoxxxxISCSI:vm-1014-disk-2,size=4G
	// description:Base image
	// cores:2 ostype:l26

	name := ""
	if _, isSet := vmConfig["name"]; isSet {
		name = vmConfig["name"].(string)
	}
	description := ""
	if _, isSet := vmConfig["description"]; isSet {
		description = vmConfig["description"].(string)
	}
	onboot := true
	if _, isSet := vmConfig["onboot"]; isSet {
		onboot = Itob(int(vmConfig["onboot"].(float64)))
	}
	agent := "1"
	if a, isSet := vmConfig["agent"]; isSet {
		// this is needed to handle 5.x PVE where the parameter is only 1 or 0
		switch a.(type) {
		case float64:
			agent = fmt.Sprintf("%.0f", a.(float64))
		default:
			agent = a.(string)
		}
	}
	ostype := "other"
	if _, isSet := vmConfig["ostype"]; isSet {
		ostype = vmConfig["ostype"].(string)
	}
	memory := 0.0
	if _, isSet := vmConfig["memory"]; isSet {
		memory = vmConfig["memory"].(float64)
	}
	cores := 1.0
	if _, isSet := vmConfig["cores"]; isSet {
		cores = vmConfig["cores"].(float64)
	}
	sockets := 1.0
	if _, isSet := vmConfig["sockets"]; isSet {
		sockets = vmConfig["sockets"].(float64)
	}
	config = &ConfigQemu{
		Name:        name,
		Description: strings.TrimSpace(description),
		Onboot:      onboot,
		Agent:       agent,
		Ostype:      ostype,
		Memory:      int(memory),
		Cores:       int(cores),
		Sockets:     int(sockets),
		Disk:        VmDevices{},
		Net:         VmDevices{},
	}

	if vmConfig["ide2"] != nil {
		isoMatch := rxIso.FindStringSubmatch(vmConfig["ide2"].(string))
		config.Iso = isoMatch[1]
	}

	if _, isSet := vmConfig["ciuser"]; isSet {
		config.CIuser = vmConfig["ciuser"].(string)
	}
	if _, isSet := vmConfig["cipassword"]; isSet {
		config.CIpassword = vmConfig["cipassword"].(string)
	}
	if _, isSet := vmConfig["searchdomain"]; isSet {
		config.Searchdomain = vmConfig["searchdomain"].(string)
	}
	if _, isSet := vmConfig["sshkeys"]; isSet {
		config.Sshkeys, _ = url.PathUnescape(vmConfig["sshkeys"].(string))
	}
	if _, isSet := vmConfig["ipconfig0"]; isSet {
		config.Ipconfig0 = vmConfig["ipconfig0"].(string)
	}
	if _, isSet := vmConfig["ipconfig1"]; isSet {
		config.Ipconfig1 = vmConfig["ipconfig1"].(string)
	}

	// Add disks.
	diskNames := []string{}

	for k, _ := range vmConfig {
		if diskName := rxDiskName.FindStringSubmatch(k); len(diskName) > 0 {
			diskNames = append(diskNames, diskName[0])
		}
	}

	for _, diskName := range diskNames {
		diskConfStr := vmConfig[diskName]
		diskConfList := strings.Split(diskConfStr.(string), ",")

		//
		id := rxDeviceID.FindStringSubmatch(diskName)
		diskID, _ := strconv.Atoi(id[0])
		diskType := rxDiskType.FindStringSubmatch(diskName)[0]
		storageName, fileName := ParseSubConf(diskConfList[0], ":")

		//
		diskConfMap := VmDevice{
			"type":    diskType,
			"storage": storageName,
			"file":    fileName,
		}

		// Add rest of device config.
		diskConfMap.readDeviceConfig(diskConfList[1:])

		// And device config to disks map.
		if len(diskConfMap) > 0 {
			config.Disk[diskID] = diskConfMap
		}
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
		id := rxDeviceID.FindStringSubmatch(nicName)
		nicID, _ := strconv.Atoi(id[0])
		model, macaddr := ParseSubConf(nicConfList[0], "=")

		// Add model and MAC address.
		nicConfMap := VmDevice{
			"model":   model,
			"macaddr": macaddr,
		}

		// Add rest of device config.
		nicConfMap.readDeviceConfig(nicConfList[1:])

		// And device config to networks.
		if len(nicConfMap) > 0 {
			config.Net[nicID] = nicConfMap
		}
	}

	return
}

// Create parameters for each Nic device.
func (c ConfigQemu) CreateNetParams(vmID int, params map[string]interface{}) {
	for nicID, nicConfMap := range c.Net {

		nicConfParam := VmDeviceParam{}

		// Set Nic name.
		qemuNicName := "net" + strconv.Itoa(nicID)

		// Set Mac address.
		if nicConfMap["macaddr"] == nil || nicConfMap["macaddr"].(string) == "" {
			// Generate Mac based on VmID and NicID so it will be the same always.
			macaddr := make(net.HardwareAddr, 6)
			rand.Seed(time.Now().UnixNano())
			rand.Read(macaddr)
			macaddr[0] = (macaddr[0] | 2) & 0xfe // fix from github issue #18
			macAddrUppr := strings.ToUpper(fmt.Sprintf("%v", macaddr))
			// use model=mac format for older proxmox compatability
			macAddr := fmt.Sprintf("%v=%v", nicConfMap["model"], macAddrUppr)

			// Add Mac to source map so it will be returned. (useful for some use case like Terraform)
			nicConfMap["macaddr"] = macAddrUppr
			// and also add it to the parameters which will be sent to Proxmox API.
			nicConfParam = append(nicConfParam, macAddr)
		} else {
			macAddr := fmt.Sprintf("%v=%v", nicConfMap["model"], nicConfMap["macaddr"].(string))
			nicConfParam = append(nicConfParam, macAddr)
		}

		// Set bridge if not nat.
		if nicConfMap["bridge"].(string) != "nat" {
			bridge := fmt.Sprintf("bridge=%v", nicConfMap["bridge"])
			nicConfParam = append(nicConfParam, bridge)
		}

		// Keys that are not used as real/direct conf.
		ignoredKeys := []string{"id", "bridge", "macaddr", "model"}

		// Rest of config.
		nicConfParam = nicConfParam.createDeviceParam(nicConfMap, ignoredKeys)

		// Add nic to Qemu prams.
		params[qemuNicName] = strings.Join(nicConfParam, ",")
	}

	return
}

// Create parameters for each disk.
func (c ConfigQemu) CreateDisksParams(
	vmID int,
	params map[string]interface{},
	cloned bool,
) {
	for diskID, diskConfMap := range c.Disk {

		// skip the first disk for clones (may not always be right, but a template probably has at least 1 disk)
		if diskID == 0 && cloned {
			continue
		}
		diskConfParam := VmDeviceParam{
			"media=disk",
		}

		// Device name.
		deviceType := diskConfMap["type"].(string)
		qemuDiskName := deviceType + strconv.Itoa(diskID)

		// Set disk storage.
		// Disk size.
		diskSizeGB := fmt.Sprintf("size=%v", diskConfMap["size"])
		diskConfParam = append(diskConfParam, diskSizeGB)

		// Disk name.
		var diskFile string
		// Currently ZFS local, LVM, and Directory are considered.
		// Other formats are not verified, but could be added if they're needed.
		rxStorageTypes := `(zfspool|lvm)`
		storageType := diskConfMap["storage_type"].(string)
		if matched, _ := regexp.MatchString(rxStorageTypes, storageType); matched {
			diskFile = fmt.Sprintf("file=%v:vm-%v-disk-%v", diskConfMap["storage"], vmID, diskID)
		} else {
			diskFile = fmt.Sprintf("file=%v:%v/vm-%v-disk-%v.%v", diskConfMap["storage"], vmID, vmID, diskID, diskConfMap["format"])
		}
		diskConfParam = append(diskConfParam, diskFile)

		// Set cache if not none (default).
		if diskConfMap["cache"].(string) != "none" {
			diskCache := fmt.Sprintf("cache=%v", diskConfMap["cache"])
			diskConfParam = append(diskConfParam, diskCache)
		}

		// Keys that are not used as real/direct conf.
		ignoredKeys := []string{"id", "type", "storage", "storage_type", "size", "cache"}

		// Rest of config.
		diskConfParam = diskConfParam.createDeviceParam(diskConfMap, ignoredKeys)

		// Add back to Qemu prams.
		params[qemuDiskName] = strings.Join(diskConfParam, ",")
	}

	return
}
