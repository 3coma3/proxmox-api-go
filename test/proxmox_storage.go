package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
)

func init() {
	// factory
	testActions["storage_newstorage"] = errNotImplemented

	testActions["storage_getstoragelist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetStorageList()
	}

	testActions["storage_findstorage"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.FindStorage(options.Args[1])
	}

	testActions["storage_check"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return nil, proxmox.NewStorage(options.Args[1]).Check()
	}

	testActions["storage_getinfo"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.NewStorage(options.Args[1]).GetInfo()
	}

}
