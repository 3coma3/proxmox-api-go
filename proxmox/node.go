package proxmox

func GetNodeList(client *Client) (list map[string]interface{}, err error) {
	err = client.GetJsonRetryable("/nodes", &list, 3)
	return
}
