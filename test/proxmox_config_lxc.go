package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"encoding/json"
	"os"
)

func init() {
	testActions["configlxc_createvm"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		if config, err := proxmox.NewConfigLxcFromJson(os.Stdin, false); err == nil {
			vm.SetNode(proxmox.NewNode(options.Args[1]))
			err = config.CreateVm(vm)
		}

		return
	}

	testActions["configlxc_updateconfig"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		if config, err := proxmox.NewConfigLxcFromJson(os.Stdin, true); err == nil {
			if vminfo, err := vm.GetInfo(); err == nil {
				vm.SetNode(proxmox.NewNode(vminfo["node"].(string)))
				vm.SetType(vminfo["type"].(string))
				err = config.UpdateConfig(vm)
			}
		}

		return
	}

	testActions["configlxc_newconfiglxcfromjson"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.NewConfigLxcFromJson(os.Stdin, false)
	}

	testActions["configlxc_newconfiglxcfromapi"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return proxmox.NewConfigLxcFromApi(vm)
	}

	testActions["configlxc_createnetparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		if err = json.NewDecoder(os.Stdin).Decode(&inputparams); err == nil {

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
			config.CreateNetParams(options.VMid, premap)
			response = premap
		}

		return
	}

	testActions["configlxc_creatempparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the mountpoints is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		if err = json.NewDecoder(os.Stdin).Decode(&inputparams); err == nil {

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
			config.CreateDisksParams(options.VMid, premap, false)
			response = premap
		}

		return
	}
}
