package proxmox

import (
	"errors"
	"fmt"
)

type Storage struct {
	name        string
	storagetype string
	config      *map[string]interface{}
}

// base factory
func NewStorage(name string) *Storage {
	return &Storage{
		name: name,
	}
}

func (storage *Storage) Name() string {
	return storage.name
}

func GetStorageList() (list map[string]interface{}, err error) {
	err = GetClient().GetJsonRetryable("/storage", &list, 3)
	return
}

// factory by name
// getInfo for storage already looks up by name, so use that
func FindStorage(name string) (storage *Storage, err error) {
	storage = NewStorage(name)
	if _, err = storage.GetInfo(); err != nil {
		return nil, err
	}
	return
}

func (storage *Storage) Check() (err error) {
	_, err = storage.GetInfo()
	return
}

func (storage *Storage) GetInfo() (storageInfo map[string]interface{}, err error) {
	resp, err := GetStorageList()
	storages := resp["data"].([]interface{})
	for i := range storages {
		storageInfo = storages[i].(map[string]interface{})
		if storageInfo["storage"].(string) == storage.name {
			return
		}
	}
	return nil, errors.New(fmt.Sprintf("Storage '%s' not found", storage.name))
}
