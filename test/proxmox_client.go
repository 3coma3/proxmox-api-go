package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
)

func init() {
	// move this to node code tests
	testActions["node_getnodelist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetNodeList()
	}

	// lean factories
	testActions["client_newclient"] = errNotImplemented

	// TODO
	testActions["client_getjsonretryable"] = errNotImplemented
	testActions["client_waitforcompletion"] = errNotImplemented

	testActions["client_gettaskexitstatus"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetTaskExitstatus(options.Args[1])
	}
}
