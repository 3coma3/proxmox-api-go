package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"encoding/json"
	"os"
)

func init() {
	testActions["configqemu_createvm"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		if config, err := proxmox.NewConfigQemuFromJson(os.Stdin); err == nil {
			vm.SetNode(proxmox.NewNode(options.Args[1]))
			err = config.CreateVm(vm)
		}

		return
	}

	// simple method
	testActions["configqemu_hascloudinit"] = errNotImplemented

	testActions["configqemu_updateconfig"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		if config, err := proxmox.NewConfigQemuFromJson(os.Stdin); err == nil {
			if vminfo, err := vm.GetInfo(); err == nil {
				vm.SetNode(proxmox.NewNode(vminfo["node"].(string)))
				vm.SetType(vminfo["type"].(string))
				err = config.UpdateConfig(vm)
			}
		}

		return
	}

	testActions["configqemu_newconfigqemufromjson"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.NewConfigQemuFromJson(os.Stdin)
	}

	testActions["configqemu_newconfigqemufromapi"] = func(options *TOptions) (response interface{}, err error) {
		_, v := newClientAndVmr(options)
		return proxmox.NewConfigQemuFromApi(v)
	}

	testActions["configqemu_createnetparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		if err = json.NewDecoder(os.Stdin).Decode(&inputparams); err == nil {

			// put the map as Net[0] as if it were built by
			// NewConfigQemuFromJson
			config := &proxmox.ConfigQemu{
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

	testActions["configqemu_createdisksparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the disks is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		if err = json.NewDecoder(os.Stdin).Decode(&inputparams); err == nil {

			// put the map as QemuDisks[0] as if it were built by
			// NewConfigQemuFromJson
			config := &proxmox.ConfigQemu{
				Disk: proxmox.VmDevices{0: inputparams},
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
