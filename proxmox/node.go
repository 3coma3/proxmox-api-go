package proxmox

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type Node struct {
	name string
}

// base factory
func NewNode(name string) *Node {
	return &Node{name: name}
}

func (node *Node) Name() string {
	return node.name
}

func GetNodeList() (list map[string]interface{}, err error) {
	err = GetClient().GetJsonRetryable("/nodes", &list, 3)
	return
}

// factory by name
// getInfo for nodes already looks up by name, so use that
func FindNode(name string) (node *Node, err error) {
	node = NewNode(name)
	if _, err = node.GetInfo(); err != nil {
		return nil, err
	}
	return
}

func (node *Node) Check() (err error) {
	_, err = node.GetInfo()
	return
}

func (node *Node) GetInfo() (nodeInfo map[string]interface{}, err error) {
	resp, err := GetNodeList()
	nodes := resp["data"].([]interface{})
	for i := range nodes {
		nodeInfo = nodes[i].(map[string]interface{})
		if nodeInfo["node"].(string) == node.name {
			return
		}
	}
	return nil, errors.New(fmt.Sprintf("Node '%s' not found", node.name))
}

// TODO: add autodetection of existant volumes and act accordingly
func (node *Node) CreateVolume(fullDiskName string, diskParams map[string]interface{}) (err error) {
	storageName, volumeName := GetStorageAndVolumeName(fullDiskName, ":")
	diskParams["filename"] = volumeName
	reqbody := ParamsToBody(diskParams)

	url := fmt.Sprintf("/nodes/%s/storage/%s/content", node.name, storageName)
	if resp, err := GetClient().session.Post(url, nil, nil, &reqbody); err == nil {
		if taskResponse, err := ResponseJSON(resp); err == nil {
			if diskName, containsData := taskResponse["data"]; !containsData || diskName != fullDiskName {
				return errors.New(fmt.Sprintf("Cannot create VM disk %s", fullDiskName))
			}
		}
	}

	return
}

func (node *Node) DeleteVolume(fullDiskName string) (err error) {
	storageName, volumeName := GetStorageAndVolumeName(fullDiskName, ":")
	url := fmt.Sprintf("/nodes/%s/storage/%s/content/%s", node.name, storageName, volumeName)
	_, err = GetClient().session.Delete(url, nil, nil)
	return
}

// Extract disk storage and disk volume, since disk name is saved in Proxmox with its storage.
func GetStorageAndVolumeName(
	fullDiskName string,
	separator string,
) (storageName string, volumeName string) {
	storageAndVolumeName := strings.Split(fullDiskName, separator)
	storageName, volumeName = storageAndVolumeName[0], storageAndVolumeName[1]

	// when disk type is dir, volumeName is `file=local:100/vm-100-disk-0.raw`
	re := regexp.MustCompile(`\d+/(?P<filename>\S+.\S+)`)
	match := re.FindStringSubmatch(volumeName)
	if len(match) == 2 {
		volumeName = match[1]
	}

	return storageName, volumeName
}
