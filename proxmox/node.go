package proxmox

func GetNodeList() (list map[string]interface{}, err error) {
	err = GetClient().GetJsonRetryable("/nodes", &list, 3)
	return
}
