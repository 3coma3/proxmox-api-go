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

type (
	LxcDevices     map[int]map[string]interface{}
	LxcDevice      map[string]interface{}
	LxcDeviceParam []string
)

// ConfigLxc - Proxmox API LXC options
type ConfigLxc struct {
	Arch         string                 `json:"arch"`
	Cmode        string                 `json:"cmode"`
	Console      bool                   `json:"console"`
	Cores        int                    `json:"cores"`
	Cpuunits     int                    `json:"cpuunits"`
	Description  string                 `json:"description"`
	Digest       string                 `json:"digest"`
	Hostname     string                 `json:"hostname"`
	Memory       int                    `json:"memory"`
	Mp           LxcDevices             `json:"mp"`
	Nameserver   string                 `json:"nameserver"`
	Net          LxcDevices             `json:"net"`
	Onboot       bool                   `json:"onboot"`
	Ostype       string                 `json:"ostype"`
	Ostemplate   string                 `json:"ostemplate"`
	Password     string                 `json:"password"`
	Protection   bool                   `json:"protection"`
	Rootfs       LxcDevice              `json:"rootfs"`
	Searchdomain string                 `json:"searchdomain"`
	Startup      string                 `json:"startup"`
	Sshkeys      string                 `json:"ssh-public-keys"`
	Swap         int                    `json:"swap"`
	Tty          int                    `json:"tty"`
	Unprivileged bool                   `json:"unprivileged"`
	CloneParams  map[string]interface{} `json:"clone"`
}

// CreateVm - Tell Proxmox API to make the VM
func (config ConfigLxc) CreateVm(vmr *VmRef, client *Client) (err error) {
	vmr.SetVmType("lxc")

	params := map[string]interface{}{
		"vmid":            vmr.vmId,
		"arch":            config.Arch,
		"cmode":           config.Cmode,
		"console":         config.Console,
		"cores":           config.Cores,
		"cpuunits":        config.Cpuunits,
		"description":     config.Description,
		"hostname":        config.Hostname,
		"memory":          config.Memory,
		"nameserver":      config.Nameserver,
		"onboot":          config.Onboot,
		"ostype":          config.Ostype,
		"protection":      config.Protection,
		"swap":            config.Swap,
		"searchdomain":    config.Searchdomain,
		"ostemplate":      config.Ostemplate,
		"tty":             config.Tty,
		"unprivileged":    config.Unprivileged,
		"ssh-public-keys": config.Sshkeys,
	}

	// Create mountpoints config.
	config.CreateLxcMpParams(vmr.vmId, params, false)

	// Create networks config.
	config.CreateLxcNetParams(vmr.vmId, params)

	exitStatus, err := client.CreateVm(vmr, params)
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

	config.CloneParams["newid"] = vmr.vmId

	_, err = client.CloneVm(sourceVmr, config.CloneParams)
	if err != nil {
		return
	}

	return config.UpdateConfig(vmr, client)
}

func (config ConfigLxc) UpdateConfig(vmr *VmRef, client *Client) (err error) {
	params := map[string]interface{}{}

	if config.Arch != "" {
		params["arch"] = config.Arch
	}
	if config.Description != "" {
		params["description"] = config.Description
	}
	if config.Hostname != "" {
		params["hostname"] = config.Hostname
	}
	if config.Nameserver != "" {
		params["nameserver"] = config.Nameserver
	}
	if config.Ostype != "" {
		params["ostype"] = config.Ostype
	}
	if config.Searchdomain != "" {
		params["searchdomain"] = config.Searchdomain
	}

	// Decoder.Decode uses the struct, which "always" will have its members
	// set by default to a zero value. The zero value can't be tell apart from
	// user supplied values like false or 0, so this will preclude using these
	// numeric and bool parameters as they can't be properly detected
	// The best way to avoid this is to use json.UnmarshalJSON automatic parsing
	// features with pointers and rely on a map instead of a struct.
	// An added benefit will be way less translations and boilerplate between
	// user and PVEAPI data formats (as much as 5 down to 1 at many points)

	// For now, these parameters have to be explicited in the user JSON unless
	// it's desired to use their zero value
	params["console"] = config.Console
	params["cores"] = config.Cores
	params["cpuunits"] = config.Cpuunits
	params["memory"] = config.Memory
	params["ostype"] = config.Ostype
	params["protection"] = config.Protection
	params["swap"] = config.Swap
	params["tty"] = config.Tty

	// Create mountpoints config.
	config.CreateLxcMpParams(vmr.vmId, params, true)

	// Create networks config.
	config.CreateLxcNetParams(vmr.vmId, params)

	_, err = client.SetVmConfig(vmr, params)
	return err
}

// this factory returns a new struct with the members set to defaults
func NewConfigLxc() *ConfigLxc {
	return &ConfigLxc{
		Arch:         "amd64",
		Cmode:        "tty",
		Console:      true,
		Cores:        1,
		Cpuunits:     1024,
		Description:  "",
		Digest:       "",
		Hostname:     "",
		Memory:       512,
		Mp:           LxcDevices{},
		Nameserver:   "",
		Net:          LxcDevices{},
		Onboot:       false,
		Ostype:       "unmanaged",
		Protection:   false,
		Rootfs:       LxcDevice{},
		Searchdomain: "",
		Sshkeys:      "",
		Swap:         512,
		Ostemplate:   "",
		Tty:          2,
		Unprivileged: false,
	}
}

func NewConfigLxcFromJson(io io.Reader, bare bool) (config *ConfigLxc, err error) {
	if bare {
		config = &ConfigLxc{}
	} else {
		config = NewConfigLxc()
	}

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
	if _, isSet := vmConfig["cpuunits"]; isSet {
		config.Cpuunits = int(vmConfig["cpuunits"].(float64))
	}
	if _, isSet := vmConfig["description"]; isSet {
		config.Description = vmConfig["description"].(string)
	}
	if _, isSet := vmConfig["digest"]; isSet {
		config.Digest = vmConfig["digest"].(string)
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
		rootFsConfList := strings.Split(vmConfig["rootfs"].(string), ",")
		config.Rootfs["storage"], config.Rootfs["file"] = ParseSubConf(rootFsConfList[0], ":")
		config.Rootfs.readDeviceConfig(rootFsConfList[1:])
	}
	if _, isSet := vmConfig["searchdomain"]; isSet {
		config.Searchdomain = vmConfig["searchdomain"].(string)
	}
	if _, isSet := vmConfig["swap"]; isSet {
		config.Swap = int(vmConfig["swap"].(float64))
	}
	if _, isSet := vmConfig["ostemplate"]; isSet {
		config.Ostemplate = vmConfig["ostemplate"].(string)
	}
	if _, isSet := vmConfig["tty"]; isSet {
		config.Tty = int(vmConfig["tty"].(float64))
	}
	if _, isSet := vmConfig["unprivileged"]; isSet {
		config.Unprivileged = Itob(int(vmConfig["unprivileged"].(float64)))
	}

	if _, isSet := vmConfig["ssh-public-keys"]; isSet {
		config.Sshkeys, _ = url.PathUnescape(vmConfig["ssh-public-keys"].(string))
	}

	// Add mountpoints
	mpNameRe := regexp.MustCompile(`mp\d+`)
	mps := []string{}
	for k, _ := range vmConfig {
		if mpName := mpNameRe.FindStringSubmatch(k); len(mpName) > 0 {
			mps = append(mps, mpName[0])
		}
	}

	for _, mpName := range mps {
		mpConfList := strings.Split(vmConfig[mpName].(string), ",")

		mpConfMap := LxcDevice{}
		mpConfMap["storage"], mpConfMap["file"] = ParseSubConf(mpConfList[0], ":")

		mpConfMap.readDeviceConfig(mpConfList[1:])

		if len(mpConfMap) > 0 {
			id := rxDeviceID.FindStringSubmatch(mpName)
			mpID, _ := strconv.Atoi(id[0])
			config.Mp[mpID] = mpConfMap
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
		nicConfList := strings.Split(vmConfig[nicName].(string), ",")

		nicConfMap := LxcDevice{}
		nicConfMap.readDeviceConfig(nicConfList)

		if len(nicConfMap) > 0 {
			id := rxDeviceID.FindStringSubmatch(nicName)
			nicID, _ := strconv.Atoi(id[0])
			config.Net[nicID] = nicConfMap
		}
	}

	return config, nil
}

// Create parameters for each Nic device.
func (c ConfigLxc) CreateLxcNetParams(vmID int, params map[string]interface{}) error {
	for nicID, nicConfMap := range c.Net {

		nicConfParam := LxcDeviceParam{}

		// Set Nic name.
		lxcNicName := "net" + strconv.Itoa(nicID)

		// Set Mac address.
		if nicConfMap["hwaddr"] == nil || nicConfMap["hwaddr"].(string) == "" {
			// Generate Mac based on VmID and NicID so it will be the same always.
			macaddr := make(net.HardwareAddr, 6)
			rand.Seed(time.Now().UnixNano())
			rand.Read(macaddr)
			macaddr[0] = (macaddr[0] | 2) & 0xfe // fix from github issue #18
			macAddrUppr := strings.ToUpper(fmt.Sprintf("%v", macaddr))
			// use model=mac format for older proxmox compatability
			macAddr := fmt.Sprintf("hwaddr=%v", macAddrUppr)

			// Add Mac to source map so it will be returned. (useful for some use case like Terraform)
			nicConfMap["hwaddr"] = macAddrUppr
			// and also add it to the parameters which will be sent to Proxmox API.
			nicConfParam = append(nicConfParam, macAddr)
		} else {
			macAddr := fmt.Sprintf("hwaddr=%v", nicConfMap["hwaddr"].(string))
			nicConfParam = append(nicConfParam, macAddr)
		}

		// Set bridge if not nat.
		if nicConfMap["bridge"].(string) != "nat" {
			bridge := fmt.Sprintf("bridge=%v", nicConfMap["bridge"])
			nicConfParam = append(nicConfParam, bridge)
		}

		// Keys that are not used as real/direct conf.
		ignoredKeys := []string{"id", "bridge", "hwaddr", "model"}

		// Rest of config.
		nicConfParam = nicConfParam.createDeviceParam(nicConfMap, ignoredKeys)

		// Add nic to Lxc prams.
		params[lxcNicName] = strings.Join(nicConfParam, ",")
	}

	return nil
}

// Create parameters for each mountpoint
func (c ConfigLxc) CreateLxcMpParams(
	vmID int,
	params map[string]interface{},
	cloned bool,
) error {
	diskConfStr := func(diskID int, diskConfMap LxcDevice) string {
		diskConfParam := LxcDeviceParam{}

		// disk size
		diskSizeGB := fmt.Sprintf("size=%v", diskConfMap["size"])
		diskConfParam = append(diskConfParam, diskSizeGB)

		// full disk name, this is of the form storage:filename
		// if the filename parameter is defined in JSON (comes from user input),
		// set it from that (volumes must been previously set up)
		var diskFile string
		if fileName, ok := diskConfMap["filename"]; ok {
			diskFile = fmt.Sprintf("volume=%v:%v", diskConfMap["storage"], fileName.(string))
		} else {
			// for automatic creation the filename index is hardcoded
			// TODO: add autodetection of existant volumes and act accordingly
			if diskID == 1 {
				diskSize := diskConfMap["size"].(string)
				// the format for rootfs automatic creation seems to be
				// undocumented
				diskFile = fmt.Sprintf("%v:%v", diskConfMap["storage"], diskSize[:(strings.IndexAny(diskSize, "G"))])
			} else {
				// note that automatic creation for mp volumes will make CT
				// creation fail if they aren't formatted
				// TODO: add automatic formatting / fs specification
				diskFile = fmt.Sprintf("volume=%v:vm-%v-disk-%v", diskConfMap["storage"], vmID, diskID)
			}
		}
		diskConfParam = append(diskConfParam, diskFile)

		// keys that are not used as real/direct conf (or have been added above)
		ignoredKeys := []string{"id", "storage", "size", "filename"}

		// rest of config
		diskConfParam = diskConfParam.createDeviceParam(diskConfMap, ignoredKeys)

		return strings.Join(diskConfParam, ",")
	}

	// don't set up rootfs if it isn't defined
	if c.Rootfs != nil {
		params["rootfs"] = diskConfStr(1, c.Rootfs)
	}

	for diskID, diskConfMap := range c.Mp {
		params["mp"+strconv.Itoa(diskID)] = diskConfStr(diskID+2, diskConfMap)
	}

	return nil
}

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
