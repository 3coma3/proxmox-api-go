package proxmox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GetVmList() (list map[string]interface{}, err error) {
	err = GetClient().GetJsonRetryable("/cluster/resources?type=vm", &list, 3)
	return
}

func FindVm(vmName string) (vm *Vm, err error) {
	resp, err := GetVmList()
	vms := resp["data"].([]interface{})
	for vmii := range vms {
		vmMap := vms[vmii].(map[string]interface{})
		if vmMap["name"] != nil && vmMap["name"].(string) == vmName {
			vm = NewVm(int(vmMap["vmid"].(float64)))
			vm.node = vmMap["node"].(string)
			vm.vmtype = vmMap["type"].(string)
			return vm, err
		}
	}
	return nil, errors.New(fmt.Sprintf("Vm '%s' not found", vmName))
}

func GetMaxVmId() (max int, err error) {
	resp, err := GetVmList()
	vms := resp["data"].([]interface{})
	max = 0
	for vmii := range vms {
		vm := vms[vmii].(map[string]interface{})
		vmid := int(vm["vmid"].(float64))
		if vmid > max {
			max = vmid
		}
	}
	return
}

func GetNextVmId(currentId int) (nextId int, err error) {
	var data map[string]interface{}
	var url string
	if currentId >= 100 {
		url = fmt.Sprintf("/cluster/nextid?vmid=%d", currentId)
	} else {
		url = "/cluster/nextid"
	}
	_, err = GetClient().session.GetJSON(url, nil, nil, &data)
	if err == nil {
		if data["errors"] != nil {
			if currentId >= 100 {
				return GetNextVmId(currentId + 1)
			} else {
				return -1, errors.New("error using /cluster/nextid")
			}
		}
		nextId, err = strconv.Atoi(data["data"].(string))
	}
	return
}

// Vm - virtual machine ref parts
type Vm struct {
	id     int
	vmtype string
	node   string
}

func (vm *Vm) SetNode(n string) {
	vm.node = n
	return
}

func (vm *Vm) SetType(t string) {
	vm.vmtype = t
	return
}

func (vm *Vm) Id() int {
	return vm.id
}

func (vm *Vm) Node() string {
	return vm.node
}

func NewVm(id int) *Vm {
	return &Vm{id: id}
}

func (vm *Vm) Check() (err error) {
	if vm.node == "" || vm.vmtype == "" {
		_, err = vm.GetInfo()
	}
	return
}

func (vm *Vm) Create(vmParams map[string]interface{}) (exitStatus string, err error) {
	// Create VM disks first to ensure disks names.
	createdDisks, createdDisksErr := createDisks(vm.node, vmParams)
	if createdDisksErr != nil {
		return "", createdDisksErr
	}

	// Then create the VM itself.
	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s", vm.node, vm.vmtype)
	var resp *http.Response
	resp, err = GetClient().session.Post(url, nil, nil, &reqbody)
	defer resp.Body.Close()
	if err != nil {
		// This might not work if we never got a body. We'll ignore errors in trying to read,
		// but extract the body if possible to give any error information back in the exitStatus
		b, _ := ioutil.ReadAll(resp.Body)
		exitStatus = string(b)
		return exitStatus, err
	}

	taskResponse, err := ResponseJSON(resp)
	if err != nil {
		return "", err
	}
	exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	// Delete VM disks if the VM didn't create.
	if exitStatus != "OK" {
		deleteDisksErr := vm.DeleteDisks(createdDisks)
		if deleteDisksErr != nil {
			return "", deleteDisksErr
		}
	}

	return
}

func (vm *Vm) CreateTemplate() error {
	err := vm.Check()
	if err != nil {
		return err
	}

	reqbody := ParamsToBody(map[string]interface{}{"experimental": true})
	url := fmt.Sprintf("/nodes/%s/%s/%d/template", vm.node, vm.vmtype, vm.id)
	_, err = GetClient().session.Post(url, nil, nil, &reqbody)
	if err != nil {
		return err
	}

	return nil
}

func (vm *Vm) Clone(newid int, cloneParams map[string]interface{}) (exitStatus interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}

	cloneParams["newid"] = newid
	reqbody := ParamsToBody(cloneParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/clone", vm.node, vm.vmtype, vm.id)
	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	}
	return
}

func (vm *Vm) Delete() (exitStatus string, err error) {
	err = vm.Check()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("/nodes/%s/%s/%d", vm.node, vm.vmtype, vm.id)
	var taskResponse map[string]interface{}
	_, err = GetClient().session.RequestJSON("DELETE", url, nil, nil, nil, &taskResponse)
	exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	return
}

func (vm *Vm) GetInfo() (vmInfo map[string]interface{}, err error) {
	resp, err := GetVmList()
	vms := resp["data"].([]interface{})
	for i := range vms {
		vminfo := vms[i].(map[string]interface{})
		if int(vminfo["vmid"].(float64)) == vm.id {
			vm.node = vminfo["node"].(string)
			vm.vmtype = vminfo["type"].(string)
			return
		}
	}
	return nil, errors.New(fmt.Sprintf("Vm '%d' not found", vm.id))
}

func (vm *Vm) GetConfig() (config map[string]interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.node, vm.vmtype, vm.id)
	err = GetClient().GetJsonRetryable(url, &data, 3)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm CONFIG not readable")
	}
	config = data["data"].(map[string]interface{})
	return
}

func (vm *Vm) SetConfig(vmParams map[string]interface{}) (exitStatus interface{}, err error) {
	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.node, vm.vmtype, vm.id)

	var resp *http.Response

	// Use the POST async API to update qemu VMs, PUT (only method available)
	// for CTs
	if vm.vmtype == "qemu" {
		resp, err = GetClient().session.Post(url, nil, nil, &reqbody)
	} else {
		resp, err = GetClient().session.Put(url, nil, nil, &reqbody)
	}

	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	}
	return
}

func (vm *Vm) GetStatus() (vmState map[string]interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/status/current", vm.node, vm.vmtype, vm.id)
	err = GetClient().GetJsonRetryable(url, &data, 3)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm STATE not readable")
	}
	vmState = data["data"].(map[string]interface{})
	return
}

func (vm *Vm) SetStatus(setStatus string) (exitStatus string, err error) {
	err = vm.Check()
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/status/%s", vm.node, vm.vmtype, vm.id, setStatus)
	var taskResponse map[string]interface{}
	for i := 0; i < 3; i++ {
		_, err = GetClient().session.PostJSON(url, nil, nil, nil, &taskResponse)
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		if exitStatus == "" {
			time.Sleep(TaskStatusCheckInterval * time.Second)
		} else {
			return
		}
	}
	return
}

func (vm *Vm) Start() (exitStatus string, err error) {
	return vm.SetStatus("start")
}

func (vm *Vm) Suspend() (exitStatus string, err error) {
	return vm.SetStatus("suspend")
}

func (vm *Vm) Resume() (exitStatus string, err error) {
	return vm.SetStatus("resume")
}

func (vm *Vm) Reset() (exitStatus string, err error) {
	return vm.SetStatus("reset")
}

func (vm *Vm) Stop() (exitStatus string, err error) {
	return vm.SetStatus("stop")
}

func (vm *Vm) Shutdown() (exitStatus string, err error) {
	return vm.SetStatus("shutdown")
}

// Useful waiting for ISO install to complete
func (vm *Vm) WaitForShutdown() (err error) {
	for ii := 0; ii < 100; ii++ {
		vmState, err := vm.GetStatus()
		if err != nil {
			log.Print("Wait error:")
			log.Println(err)
		} else if vmState["status"] == "stopped" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return errors.New("Not shutdown within wait time")
}

func (vm *Vm) Migrate(migrateParams map[string]interface{}) (exitStatus interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}

	reqbody := ParamsToBody(migrateParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/migrate", vm.node, vm.vmtype, vm.id)
	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return "", err
		}
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	}
	return
}

func (vm *Vm) Rollback(snapshot string) (exitStatus string, err error) {
	err = vm.Check()
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", vm.node, vm.vmtype, vm.id, snapshot)
	var taskResponse map[string]interface{}
	_, err = GetClient().session.PostJSON(url, nil, nil, nil, &taskResponse)
	exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	return
}

// CreateVMDisk - Create single disk for VM on host node.
// TODO: add autodetection of existant volumes and act accordingly
func CreateDisk(
	nodeName string,
	storageName string,
	fullDiskName string,
	diskParams map[string]interface{},
) error {
	reqbody := ParamsToBody(diskParams)
	url := fmt.Sprintf("/nodes/%s/storage/%s/content", nodeName, storageName)
	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return err
		}
		if diskName, containsData := taskResponse["data"]; !containsData || diskName != fullDiskName {
			return errors.New(fmt.Sprintf("Cannot create VM disk %s", fullDiskName))
		}
	} else {
		return err
	}

	return nil
}

// createVMDisks - Make disks parameters and create all VM disks on host node.
// TODO: add autodetection of existant volumes and act accordingly
// TODO: merge sections for VM and CT volumes
func createDisks(
	node string,
	vmParams map[string]interface{},
) (disks []string, err error) {
	var (
		storageName  string
		volumeName   string
		fullDiskName string
		diskParams   map[string]interface{}
		createdDisks []string
	)

	vmID := vmParams["vmid"].(int)
	for deviceName, deviceConf := range vmParams {
		diskParams = map[string]interface{}{}

		// VM disks
		rxStorageModels := `(ide|sata|scsi|virtio)\d+`
		matched, _ := regexp.MatchString(rxStorageModels, deviceName)
		if matched {
			deviceConfMap := ParseConf(deviceConf.(string), ",", "=")
			// This if condition to differentiate between `disk` and `cdrom`.
			if media, containsFile := deviceConfMap["media"]; containsFile && media == "disk" {
				fullDiskName = deviceConfMap["file"].(string)
				storageName, volumeName = getStorageAndVolumeName(fullDiskName, ":")
				diskParams = map[string]interface{}{
					"vmid":     vmID,
					"filename": volumeName,
					"size":     deviceConfMap["size"],
				}
			}
		}

		// CT mount points
		// when autocreation features are added, rootfs can be added here
		rxCTVolumes := `(mp\d+)`
		matched, _ = regexp.MatchString(rxCTVolumes, deviceName)
		if matched {
			deviceConfMap := ParseConf(deviceConf.(string), ",", "=")

			fullDiskName = deviceConfMap["volume"].(string)
			storageName, volumeName = getStorageAndVolumeName(fullDiskName, ":")
			diskParams = map[string]interface{}{
				"vmid":     vmID,
				"filename": volumeName,
				"size":     deviceConfMap["size"],
			}
		}

		if len(diskParams) > 0 {
			err := CreateDisk(node, storageName, fullDiskName, diskParams)
			if err != nil {
				return createdDisks, err
			} else {
				createdDisks = append(createdDisks, fullDiskName)
			}
		}
	}

	return createdDisks, nil
}

func (vm *Vm) ResizeDisk(disk string, moreSizeGB int) (exitStatus interface{}, err error) {
	// PUT
	//disk:virtio0
	//size:+2G
	if disk == "" {
		disk = "virtio0"
	}
	size := fmt.Sprintf("+%dG", moreSizeGB)
	reqbody := ParamsToBody(map[string]interface{}{"disk": disk, "size": size})
	url := fmt.Sprintf("/nodes/%s/%s/%d/resize", vm.node, vm.vmtype, vm.id)
	resp, err := GetClient().session.Put(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	}
	return
}

// DeleteDisks - Delete VM disks from host node.
// By default the VM disks are deteled when the VM is deleted,
// so mainly this is used to delete the disks in case VM creation didn't complete.
func (vm *Vm) DeleteDisks(disks []string) error {
	for _, fullDiskName := range disks {
		storageName, volumeName := getStorageAndVolumeName(fullDiskName, ":")
		url := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", vm.node, storageName, volumeName)
		_, err := GetClient().session.Post(url, nil, nil, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// getStorageAndVolumeName - Extract disk storage and disk volume, since disk name is saved
// in Proxmox with its storage.
func getStorageAndVolumeName(
	fullDiskName string,
	separator string,
) (storageName string, diskName string) {
	storageAndVolumeName := strings.Split(fullDiskName, separator)
	storageName, volumeName := storageAndVolumeName[0], storageAndVolumeName[1]

	// when disk type is dir, volumeName is `file=local:100/vm-100-disk-0.raw`
	re := regexp.MustCompile(`\d+/(?P<filename>\S+.\S+)`)
	match := re.FindStringSubmatch(volumeName)
	if len(match) == 2 {
		volumeName = match[1]
	}

	return storageName, volumeName
}

func (vm *Vm) GetSpiceProxy() (vmSpiceProxy map[string]interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/spiceproxy", vm.node, vm.vmtype, vm.id)
	_, err = GetClient().session.PostJSON(url, nil, nil, nil, &data)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm SpiceProxy not readable")
	}
	vmSpiceProxy = data["data"].(map[string]interface{})
	return
}

func (vm *Vm) MonitorCmd(command string) (monitorRes map[string]interface{}, err error) {
	err = vm.Check()
	if err != nil {
		return nil, err
	}
	reqbody := ParamsToBody(map[string]interface{}{"command": command})
	url := fmt.Sprintf("/nodes/%s/%s/%d/monitor", vm.node, vm.vmtype, vm.id)
	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
	monitorRes, err = ResponseJSON(resp)
	return
}

type AgentNetworkInterface struct {
	MACAddress  string
	IPAddresses []net.IP
	Name        string
	Statistics  map[string]int64
}

func (vm *Vm) SendKeysString(keys string) (err error) {
	vmState, err := vm.GetStatus()
	if err != nil {
		return err
	}
	if vmState["status"] == "stopped" {
		return errors.New("VM must be running first")
	}
	for _, r := range keys {
		c := string(r)
		lower := strings.ToLower(c)
		if c != lower {
			c = "shift-" + lower
		} else {
			switch c {
			case "!":
				c = "shift-1"
			case "@":
				c = "shift-2"
			case "#":
				c = "shift-3"
			case "$":
				c = "shift-4"
			case "%%":
				c = "shift-5"
			case "^":
				c = "shift-6"
			case "&":
				c = "shift-7"
			case "*":
				c = "shift-8"
			case "(":
				c = "shift-9"
			case ")":
				c = "shift-0"
			case "_":
				c = "shift-minus"
			case "+":
				c = "shift-equal"
			case " ":
				c = "spc"
			case "/":
				c = "slash"
			case "\\":
				c = "backslash"
			case ",":
				c = "comma"
			case "-":
				c = "minus"
			case "=":
				c = "equal"
			case ".":
				c = "dot"
			case "?":
				c = "shift-slash"
			}
		}
		_, err = vm.MonitorCmd("sendkey " + c)
		if err != nil {
			return err
		}
		time.Sleep(100)
	}
	return nil
}

// This is because proxmox create/config API won't let us make usernet devices
func (vm *Vm) SshForwardUsernet() (sshPort string, err error) {
	vmState, err := vm.GetStatus()
	if err != nil {
		return "", err
	}
	if vmState["status"] == "stopped" {
		return "", errors.New("VM must be running first")
	}
	sshPort = strconv.Itoa(vm.Id() + 22000)
	_, err = vm.MonitorCmd("netdev_add user,id=net1,hostfwd=tcp::" + sshPort + "-:22")
	if err != nil {
		return "", err
	}
	_, err = vm.MonitorCmd("device_add virtio-net-pci,id=net1,netdev=net1,addr=0x13")
	if err != nil {
		return "", err
	}
	return
}

// device_del net1
// netdev_del net1
func (vm *Vm) RemoveSshForwardUsernet() (err error) {
	vmState, err := vm.GetStatus()
	if err != nil {
		return err
	}
	if vmState["status"] == "stopped" {
		return errors.New("VM must be running first")
	}
	_, err = vm.MonitorCmd("device_del net1")
	if err != nil {
		return err
	}
	_, err = vm.MonitorCmd("netdev_del net1")
	if err != nil {
		return err
	}
	return nil
}

func (a *AgentNetworkInterface) UnmarshalJSON(b []byte) error {
	var intermediate struct {
		HardwareAddress string `json:"hardware-address"`
		IPAddresses     []struct {
			IPAddress     string `json:"ip-address"`
			IPAddressType string `json:"ip-address-type"`
			Prefix        int    `json:"prefix"`
		} `json:"ip-addresses"`
		Name       string           `json:"name"`
		Statistics map[string]int64 `json:"statistics"`
	}
	err := json.Unmarshal(b, &intermediate)
	if err != nil {
		return err
	}

	a.IPAddresses = make([]net.IP, len(intermediate.IPAddresses))
	for idx, ip := range intermediate.IPAddresses {
		a.IPAddresses[idx] = net.ParseIP(ip.IPAddress)
		if a.IPAddresses[idx] == nil {
			return fmt.Errorf("Could not parse %s as IP", ip.IPAddress)
		}
	}
	a.MACAddress = intermediate.HardwareAddress
	a.Name = intermediate.Name
	a.Statistics = intermediate.Statistics
	return nil
}

func (vm *Vm) GetAgentNetworkInterfaces() ([]AgentNetworkInterface, error) {
	var ifs []AgentNetworkInterface
	err := vm.Check()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/agent/%s", vm.node, vm.vmtype, vm.id, "network-get-interfaces")
	resp, err := GetClient().session.Get(url, nil, nil)
	if err != nil {
		return nil, err
	}

	err = TypedResponse(resp, &ifs)
	return ifs, err
}
