package proxmox

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func GetVmList(client *Client) (list map[string]interface{}, err error) {
	err = client.GetJsonRetryable("/cluster/resources?type=vm", &list, 3)
	return
}

func FindVm(client *Client, vmName string) (vm *Vm, err error) {
	resp, err := GetVmList(client)
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

func GetNextVmId(client *Client, currentId int) (nextId int, err error) {
	var data map[string]interface{}
	var url string
	if currentId >= 100 {
		url = fmt.Sprintf("/cluster/nextid?vmid=%d", currentId)
	} else {
		url = "/cluster/nextid"
	}
	_, err = client.session.GetJSON(url, nil, nil, &data)
	if err == nil {
		if data["errors"] != nil {
			if currentId >= 100 {
				return GetNextVmId(client, currentId+1)
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

func (v *Vm) SetNode(n string) {
	v.node = n
	return
}

func (v *Vm) SetType(t string) {
	v.vmtype = t
	return
}

func (v *Vm) Id() int {
	return v.id
}

func (v *Vm) Node() string {
	return v.node
}

func NewVm(id int) *Vm {
	return &Vm{id: id}
}

func (v *Vm) Check(client *Client) (err error) {
	if v.node == "" || v.vmtype == "" {
		_, err = v.GetInfo(client)
	}
	return
}

func (v *Vm) Create(client *Client, vmParams map[string]interface{}) (exitStatus string, err error) {
	// Create VM disks first to ensure disks names.
	createdDisks, createdDisksErr := createDisks(client, v.node, vmParams)
	if createdDisksErr != nil {
		return "", createdDisksErr
	}

	// Then create the VM itself.
	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s", v.node, v.vmtype)
	var resp *http.Response
	resp, err = client.session.Post(url, nil, nil, &reqbody)
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
	exitStatus, err = client.WaitForCompletion(taskResponse)
	// Delete VM disks if the VM didn't create.
	if exitStatus != "OK" {
		deleteDisksErr := v.DeleteDisks(client, createdDisks)
		if deleteDisksErr != nil {
			return "", deleteDisksErr
		}
	}

	return
}

func (v *Vm) Clone(client *Client, newid int, cloneParams map[string]interface{}) (exitStatus interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}

	cloneParams["newid"] = newid
	reqbody := ParamsToBody(cloneParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/clone", v.node, v.vmtype, v.id)
	resp, err := client.session.Post(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = client.WaitForCompletion(taskResponse)
	}
	return
}

func (v *Vm) CreateTemplate(client *Client) error {
	err := v.Check(client)
	if err != nil {
		return err
	}

	reqbody := ParamsToBody(map[string]interface{}{"experimental": true})
	url := fmt.Sprintf("/nodes/%s/%s/%d/template", v.node, v.vmtype, v.id)
	_, err = client.session.Post(url, nil, nil, &reqbody)
	if err != nil {
		return err
	}

	return nil
}

func (v *Vm) Delete(client *Client) (exitStatus string, err error) {
	err = v.Check(client)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("/nodes/%s/%s/%d", v.node, v.vmtype, v.id)
	var taskResponse map[string]interface{}
	_, err = client.session.RequestJSON("DELETE", url, nil, nil, nil, &taskResponse)
	exitStatus, err = client.WaitForCompletion(taskResponse)
	return
}

func (v *Vm) GetInfo(client *Client) (vmInfo map[string]interface{}, err error) {
	resp, err := GetVmList(client)
	vms := resp["data"].([]interface{})
	for vmii := range vms {
		vm := vms[vmii].(map[string]interface{})
		if int(vm["vmid"].(float64)) == v.id {
			vmInfo = vm
			v.node = vmInfo["node"].(string)
			v.vmtype = vmInfo["type"].(string)
			return
		}
	}
	return nil, errors.New(fmt.Sprintf("Vm '%d' not found", v.id))
}

func (v *Vm) GetConfig(client *Client) (config map[string]interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/config", v.node, v.vmtype, v.id)
	err = client.GetJsonRetryable(url, &data, 3)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm CONFIG not readable")
	}
	config = data["data"].(map[string]interface{})
	return
}

func (v *Vm) SetConfig(client *Client, vmParams map[string]interface{}) (exitStatus interface{}, err error) {
	reqbody := ParamsToBody(vmParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/config", v.node, v.vmtype, v.id)

	var resp *http.Response

	// Use the POST async API to update qemu VMs, PUT (only method available)
	// for CTs
	if v.vmtype == "qemu" {
		resp, err = client.session.Post(url, nil, nil, &reqbody)
	} else {
		resp, err = client.session.Put(url, nil, nil, &reqbody)
	}

	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = client.WaitForCompletion(taskResponse)
	}
	return
}

func (v *Vm) GetStatus(client *Client) (vmState map[string]interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/status/current", v.node, v.vmtype, v.id)
	err = client.GetJsonRetryable(url, &data, 3)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm STATE not readable")
	}
	vmState = data["data"].(map[string]interface{})
	return
}

func (v *Vm) SetStatus(client *Client, setStatus string) (exitStatus string, err error) {
	err = v.Check(client)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/status/%s", v.node, v.vmtype, v.id, setStatus)
	var taskResponse map[string]interface{}
	for i := 0; i < 3; i++ {
		_, err = client.session.PostJSON(url, nil, nil, nil, &taskResponse)
		exitStatus, err = client.WaitForCompletion(taskResponse)
		if exitStatus == "" {
			time.Sleep(TaskStatusCheckInterval * time.Second)
		} else {
			return
		}
	}
	return
}

func (v *Vm) Start(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "start")
}

func (v *Vm) Stop(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "stop")
}

func (v *Vm) Shutdown(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "shutdown")
}

func (v *Vm) Reset(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "reset")
}

func (v *Vm) Suspend(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "suspend")
}

func (v *Vm) Resume(client *Client) (exitStatus string, err error) {
	return v.SetStatus(client, "resume")
}

func (v *Vm) Migrate(client *Client, migrateParams map[string]interface{}) (exitStatus interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}

	reqbody := ParamsToBody(migrateParams)
	url := fmt.Sprintf("/nodes/%s/%s/%d/migrate", v.node, v.vmtype, v.id)
	resp, err := client.session.Post(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return "", err
		}
		exitStatus, err = client.WaitForCompletion(taskResponse)
	}
	return
}

func (v *Vm) Rollback(client *Client, snapshot string) (exitStatus string, err error) {
	err = v.Check(client)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("/nodes/%s/%s/%d/snapshot/%s/rollback", v.node, v.vmtype, v.id, snapshot)
	var taskResponse map[string]interface{}
	_, err = client.session.PostJSON(url, nil, nil, nil, &taskResponse)
	exitStatus, err = client.WaitForCompletion(taskResponse)
	return
}

// CreateVMDisk - Create single disk for VM on host node.
// TODO: add autodetection of existant volumes and act accordingly
func CreateDisk(
	client *Client,
	nodeName string,
	storageName string,
	fullDiskName string,
	diskParams map[string]interface{},
) error {
	reqbody := ParamsToBody(diskParams)
	url := fmt.Sprintf("/nodes/%s/storage/%s/content", nodeName, storageName)
	resp, err := client.session.Post(url, nil, nil, &reqbody)
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
	client *Client,
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
			err := CreateDisk(client, node, storageName, fullDiskName, diskParams)
			if err != nil {
				return createdDisks, err
			} else {
				createdDisks = append(createdDisks, fullDiskName)
			}
		}
	}

	return createdDisks, nil
}

func (v *Vm) ResizeDisk(client *Client, disk string, moreSizeGB int) (exitStatus interface{}, err error) {
	// PUT
	//disk:virtio0
	//size:+2G
	if disk == "" {
		disk = "virtio0"
	}
	size := fmt.Sprintf("+%dG", moreSizeGB)
	reqbody := ParamsToBody(map[string]interface{}{"disk": disk, "size": size})
	url := fmt.Sprintf("/nodes/%s/%s/%d/resize", v.node, v.vmtype, v.id)
	resp, err := client.session.Put(url, nil, nil, &reqbody)
	if err == nil {
		taskResponse, err := ResponseJSON(resp)
		if err != nil {
			return nil, err
		}
		exitStatus, err = client.WaitForCompletion(taskResponse)
	}
	return
}

// DeleteDisks - Delete VM disks from host node.
// By default the VM disks are deteled when the VM is deleted,
// so mainly this is used to delete the disks in case VM creation didn't complete.
func (v *Vm) DeleteDisks(
	client *Client,
	disks []string,
) error {
	for _, fullDiskName := range disks {
		storageName, volumeName := getStorageAndVolumeName(fullDiskName, ":")
		url := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", v.node, storageName, volumeName)
		_, err := client.session.Post(url, nil, nil, nil)
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

func (v *Vm) GetSpiceProxy(client *Client) (vmSpiceProxy map[string]interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}
	var data map[string]interface{}
	url := fmt.Sprintf("/nodes/%s/%s/%d/spiceproxy", v.node, v.vmtype, v.id)
	_, err = client.session.PostJSON(url, nil, nil, nil, &data)
	if err != nil {
		return nil, err
	}
	if data["data"] == nil {
		return nil, errors.New("Vm SpiceProxy not readable")
	}
	vmSpiceProxy = data["data"].(map[string]interface{})
	return
}

func (v *Vm) MonitorCmd(client *Client, command string) (monitorRes map[string]interface{}, err error) {
	err = v.Check(client)
	if err != nil {
		return nil, err
	}
	reqbody := ParamsToBody(map[string]interface{}{"command": command})
	url := fmt.Sprintf("/nodes/%s/%s/%d/monitor", v.node, v.vmtype, v.id)
	resp, err := client.session.Post(url, nil, nil, &reqbody)
	monitorRes, err = ResponseJSON(resp)
	return
}

type AgentNetworkInterface struct {
	MACAddress  string
	IPAddresses []net.IP
	Name        string
	Statistics  map[string]int64
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

func (v *Vm) GetAgentNetworkInterfaces(client *Client) ([]AgentNetworkInterface, error) {
	var ifs []AgentNetworkInterface
	err := v.Check(client)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("/nodes/%s/%s/%d/agent/%s", v.node, v.vmtype, v.id, "network-get-interfaces")
	resp, err := client.session.Get(url, nil, nil)
	if err != nil {
		return nil, err
	}

	err = TypedResponse(resp, &ifs)
	return ifs, err
}
