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

type Vm struct {
	id     int
	vmtype string
	node   *Node
}

// base factory
func NewVm(id int) *Vm {
	return &Vm{id: id, node: nil, vmtype: ""}
}

func (vm *Vm) Id() int {
	return vm.id
}

func (vm *Vm) Node() *Node {
	return vm.node
}

func (vm *Vm) Type() string {
	return vm.vmtype
}

func (vm *Vm) SetNode(n *Node) {
	vm.node = n
	return
}

func (vm *Vm) SetType(t string) {
	vm.vmtype = t
	return
}

func (vm *Vm) Check() (err error) {
	var vmInfo map[string]interface{}

	if vm.node == nil || vm.vmtype == "" {
		if vmInfo, err = vm.GetInfo(); err == nil {
			vm.vmtype = vmInfo["type"].(string)
			vm.node = NewNode(vmInfo["node"].(string))
		}
	}
	return
}

func GetVmList() (list map[string]interface{}, err error) {
	err = GetClient().GetJsonRetryable("/cluster/resources?type=vm", &list, 3)
	return
}

func (vm *Vm) GetInfo() (vmInfo map[string]interface{}, err error) {
	var resp map[string]interface{}

	if resp, err = GetVmList(); err != nil {
		return
	}

	vms := resp["data"].([]interface{})
	for i := range vms {
		vmInfo = vms[i].(map[string]interface{})
		if int(vmInfo["vmid"].(float64)) == vm.id {
			return
		}
	}
	return nil, errors.New(fmt.Sprintf("Vm '%d' not found", vm.id))
}

// factory by name
func FindVm(name string) (vm *Vm, err error) {
	var resp map[string]interface{}
	if resp, err = GetVmList(); err == nil {
		vms := resp["data"].([]interface{})
		for i := range vms {
			vmInfo := vms[i].(map[string]interface{})
			if vmInfo["name"] != nil && vmInfo["name"].(string) == name {
				vm = NewVm(int(vmInfo["vmid"].(float64)))
				vm.node = NewNode(vmInfo["node"].(string))
				vm.vmtype = vmInfo["type"].(string)
				return
			}
		}
	}

	return nil, errors.New(fmt.Sprintf("Vm '%s' not found", name))
}

func GetMaxVmId() (max int, err error) {
	if resp, err := GetVmList(); err == nil {
		vms := resp["data"].([]interface{})
		max = 0
		for vmii := range vms {
			vm := vms[vmii].(map[string]interface{})
			if vmid := int(vm["vmid"].(float64)); vmid > max {
				max = vmid
			}
		}
	}

	return
}

func GetNextVmId(currentId int) (nextId int, err error) {
	var (
		data map[string]interface{}
		url  string
	)

	if currentId >= 100 {
		url = fmt.Sprintf("/cluster/nextid?vmid=%d", currentId)
	} else {
		url = "/cluster/nextid"
	}

	if _, err = GetClient().session.GetJSON(url, nil, nil, &data); err == nil {
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

func (vm *Vm) Create(vmParams map[string]interface{}) (exitStatus string, err error) {
	// Create VM disks first to ensure disks names.
	createdDisks, createdDisksErr := vm.createDisks(vmParams)
	if createdDisksErr != nil {
		return "", createdDisksErr
	}

	// Then create the VM itself.
	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s", vm.node.name, vm.vmtype)

	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
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
		deleteDisksErr := vm.deleteDisks(createdDisks)
		if deleteDisksErr != nil {
			return "", deleteDisksErr
		}
	}

	return
}

func (vm *Vm) CreateTemplate() (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/template", vm.node.name, vm.vmtype, vm.id)
	return GetClient().session.Post(url, nil, nil, nil)
}

func (vm *Vm) Clone(newid int, cloneParams map[string]interface{}) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	if newid > 0 {
		cloneParams["newid"] = newid
	}

	reqbody := ParamsToBody(cloneParams)

	url := fmt.Sprintf("/nodes/%s/%s/%d/clone", vm.node.name, vm.vmtype, vm.id)
	if resp, err := GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			return GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

func (vm *Vm) Delete() (exitStatus string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d", vm.node.name, vm.vmtype, vm.id)
	var taskResponse map[string]interface{}
	if _, err = GetClient().session.RequestJSON("DELETE", url, nil, nil, nil, &taskResponse); err == nil {
		return GetClient().WaitForCompletion(taskResponse)
	}

	return
}

func (vm *Vm) GetConfig() (config map[string]interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.node.name, vm.vmtype, vm.id)
	var resp map[string]interface{}
	if err = GetClient().GetJsonRetryable(url, &resp, 3); err == nil {
		if resp["data"] == nil {
			return nil, errors.New("Vm config could not be read")
		}
		config = resp["data"].(map[string]interface{})
	}

	return
}

func (vm *Vm) SetConfig(vmParams map[string]interface{}) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/config", vm.node.name, vm.vmtype, vm.id)
	var resp *http.Response

	// Use the POST async API to update qemu VMs, PUT for CTs
	if vm.vmtype == "qemu" {
		resp, err = GetClient().session.Post(url, nil, nil, &reqbody)
	} else {
		resp, err = GetClient().session.Put(url, nil, nil, &reqbody)
	}

	if err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}
	return
}

func (vm *Vm) GetStatus() (vmState map[string]interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/status/current", vm.node.name, vm.vmtype, vm.id)
	var resp map[string]interface{}
	if err = GetClient().GetJsonRetryable(url, &resp, 3); err == nil {
		if resp["data"] == nil {
			return nil, errors.New("Vm status could not be read")
		}
		vmState = resp["data"].(map[string]interface{})
	}

	return
}

func (vm *Vm) SetStatus(setStatus string) (exitStatus string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/status/%s", vm.node.name, vm.vmtype, vm.id, setStatus)
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
	if err = vm.Check(); err != nil {
		return
	}

	for ii := 0; ii < 100; ii++ {
		if vmStatus, err := vm.GetStatus(); err != nil {
			log.Print("Wait error:")
			log.Println(err)
		} else if vmStatus["status"] == "stopped" {
			return nil
		}
		time.Sleep(5 * time.Second)
	}

	return errors.New("Not shutdown within wait time")
}

func (vm *Vm) Migrate(migrateParams map[string]interface{}) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(migrateParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/migrate", vm.node.name, vm.vmtype, vm.id)
	if resp, err := GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

func (vm *Vm) GetSnapshotList() (list map[string]interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/", vm.node.name, vm.vmtype, vm.id)
	err = GetClient().GetJsonRetryable(url, &list, 3)

	return
}

func (vm *Vm) CreateSnapshot(snapParams map[string]interface{}) (exitStatus string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(snapParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot", vm.node.name, vm.vmtype, vm.id)

	var (
		resp         *http.Response
		taskResponse map[string]interface{}
	)

	if resp, err = GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err = ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

func (vm *Vm) DeleteSnapshot(snapName string) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s", vm.node.name, vm.vmtype, vm.id, snapName)
	return GetClient().session.Delete(url, nil, nil)
}

func (vm *Vm) Rollback(snapName string) (exitStatus string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", vm.node.name, vm.vmtype, vm.id, snapName)
	var taskResponse map[string]interface{}
	if _, err = GetClient().session.PostJSON(url, nil, nil, nil, &taskResponse); err == nil {
		exitStatus, err = GetClient().WaitForCompletion(taskResponse)
	}

	return
}

func (vm *Vm) CreateBackup(bkpParams map[string]interface{}) (exitStatus string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	bkpParams["vmid"] = vm.id
	reqbody := ParamsToBody(bkpParams)
	url := fmt.Sprintf("/nodes/%s/vzdump", vm.node.name)
	if resp, err := GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

// createDisks - Make disks parameters and create all VM disks on host node.
// TODO: add autodetection of existant volumes and act accordingly
// TODO: merge sections for VM and CT volumes
func (vm *Vm) createDisks(vmParams map[string]interface{}) (createdDisks []string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	for deviceName, deviceConf := range vmParams {
		var fullDiskName string
		diskParams := map[string]interface{}{}

		// VM disks
		rxStorageModels := `(ide|sata|scsi|virtio)\d+`
		matched, _ := regexp.MatchString(rxStorageModels, deviceName)
		if matched {
			deviceConfMap := ParseConf(deviceConf.(string), ",", "=")
			// exclude `cdrom`
			if media, containsFile := deviceConfMap["media"]; containsFile && media == "disk" {
				fullDiskName = deviceConfMap["file"].(string)
				diskParams = map[string]interface{}{
					"vmid": vm.id,
					"size": deviceConfMap["size"],
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
			diskParams = map[string]interface{}{
				"vmid": vm.id,
				"size": deviceConfMap["size"],
			}
		}

		if len(diskParams) > 0 {
			if err = vm.node.CreateVolume(fullDiskName, diskParams); err == nil {
				createdDisks = append(createdDisks, fullDiskName)
				continue
			}
			break
		}
	}

	return
}

func (vm *Vm) MoveDisk(moveParams map[string]interface{}) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(moveParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/move_", vm.node.name, vm.vmtype, vm.id)
	if vm.vmtype == "qemu" {
		url += "disk"
	} else {
		url += "volume"
	}

	if resp, err := GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

// sizeGB can be a number to set an absolute size, or a number preceded by + to
// grow the volume by that many GB. If using an absolute size this has to be
// larger than the current size (shrinking is not supported by PVE)
func (vm *Vm) ResizeDisk(disk string, sizeGB string) (exitStatus interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(map[string]interface{}{"disk": disk, "size": sizeGB})
	url := fmt.Sprintf("/nodes/%s/%s/%d/resize", vm.node.name, vm.vmtype, vm.id)
	if resp, err := GetClient().session.Put(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			exitStatus, err = GetClient().WaitForCompletion(taskResponse)
		}
	}

	return
}

// By default the VM disks are deteled when the VM is deleted,
// so mainly this is used to delete the disks in case VM creation didn't complete.
func (vm *Vm) deleteDisks(disks []string) (err error) {
	if err = vm.Check(); err != nil {
		return
	}

	for _, fullDiskName := range disks {
		err = vm.node.DeleteVolume(fullDiskName)
	}

	return nil
}

func (vm *Vm) GetSpiceProxy() (vmSpiceProxy map[string]interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	var resp map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/spiceproxy", vm.node.name, vm.vmtype, vm.id)
	if _, err = GetClient().session.PostJSON(url, nil, nil, nil, &resp); err == nil {
		if resp["data"] == nil {
			return nil, errors.New("Vm Spice Proxy could not be read")
		}
		vmSpiceProxy = resp["data"].(map[string]interface{})
	}

	return
}

func (vm *Vm) MonitorCmd(command string) (monitorRes map[string]interface{}, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	reqbody := ParamsToBody(map[string]interface{}{"command": command})
	url := fmt.Sprintf("/nodes/%s/%s/%d/monitor", vm.node.name, vm.vmtype, vm.id)
	resp, err := GetClient().session.Post(url, nil, nil, &reqbody)
	monitorRes, err = ResponseJSON(resp)

	return
}

func (vm *Vm) SendKeysString(keys string) (err error) {
	if err = vm.Check(); err != nil {
		return
	}

	if vmStatus, err := vm.GetStatus(); err == nil {
		if vmStatus["status"] == "stopped" {
			err = errors.New("VM must be running first")
		} else {
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

				if _, err = vm.MonitorCmd("sendkey " + c); err != nil {
					break
				}

				time.Sleep(100)
			}
		}
	}

	return
}

// This is because proxmox create/config API won't let us make usernet devices
func (vm *Vm) SshForwardUsernet() (sshPort string, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	if vmStatus, err := vm.GetStatus(); err == nil {
		if vmStatus["status"] == "stopped" {
			err = errors.New("VM must be running first")
		}
	} else {
		sshPort = strconv.Itoa(vm.Id() + 22000)
		if _, err = vm.MonitorCmd("netdev_add user,id=net1,hostfwd=tcp::" + sshPort + "-:22"); err == nil {
			_, err = vm.MonitorCmd("device_add virtio-net-pci,id=net1,netdev=net1,addr=0x13")
		}
	}

	return
}

// device_del net1
// netdev_del net1
func (vm *Vm) RemoveSshForwardUsernet() (err error) {
	if err = vm.Check(); err != nil {
		return
	}

	if vmStatus, err := vm.GetStatus(); err == nil {
		if vmStatus["status"] == "stopped" {
			err = errors.New("VM must be running first")
		}
	} else if _, err = vm.MonitorCmd("device_del net1"); err == nil {
		_, err = vm.MonitorCmd("netdev_del net1")
	}

	return
}

type AgentNetworkInterface struct {
	MACAddress  string
	IPAddresses []net.IP
	Name        string
	Statistics  map[string]int64
}

func (a *AgentNetworkInterface) UnmarshalJSON(b []byte) (err error) {
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

	if err = json.Unmarshal(b, &intermediate); err == nil {
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
	}

	return
}

func (vm *Vm) GetAgentNetworkInterfaces() (ifs []AgentNetworkInterface, err error) {
	if err = vm.Check(); err != nil {
		return
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/agent/%s", vm.node.name, vm.vmtype, vm.id, "network-get-interfaces")
	if resp, err := GetClient().session.Get(url, nil, nil); err == nil {
		err = TypedResponse(resp, &ifs)
	}

	return
}
