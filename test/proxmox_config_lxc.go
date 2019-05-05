package test

import (
	"../proxmox"
	"encoding/json"
	"os"
)

func init() {
	testActions["configlxc_createvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vm := newClientAndVmr(options)

		config, err := proxmox.NewConfigLxcFromJson(os.Stdin, false)
		failOnError(err)

		vm.SetNode(options.Args[1])
		config.CreateVm(vm, client)
		return nil, nil
	}

	testActions["configlxc_updateconfig"] = func(options *TOptions) (response interface{}, err error) {
		client, vm := newClientAndVmr(options)

		config, err := proxmox.NewConfigLxcFromJson(os.Stdin, true)
		failOnError(err)

		vminfo, err := vm.GetInfo(client)
		failOnError(err)

		vm.SetNode(vminfo["node"].(string))
		vm.SetType(vminfo["type"].(string))
		config.UpdateConfig(vm, client)
		return nil, err
	}

	testActions["configlxc_newconfiglxcfromjson"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.NewConfigLxcFromJson(os.Stdin, false)
	}

	testActions["configlxc_newconfiglxcfromapi"] = func(options *TOptions) (response interface{}, err error) {
		client, vm := newClientAndVmr(options)
		return proxmox.NewConfigLxcFromApi(vm, client)
	}

	testActions["configlxc_createnetparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as Net[0] as if it were built by
		// NewConfigLxcFromJson
		config := &proxmox.ConfigLxc{
			Net: proxmox.VmDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.VmDevice{}
		return premap, config.CreateNetParams(options.VMid, premap)
	}

	testActions["configlxc_creatempparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network interfaces is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as LxcDisks[0] as if it were built by
		// NewConfigLxcFromJson
		config := &proxmox.ConfigLxc{
			Mp: proxmox.VmDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.VmDevice{}
		return premap, config.CreateMpParams(options.VMid, premap, false)
	}

}
