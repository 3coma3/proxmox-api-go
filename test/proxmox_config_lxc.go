package test

import (
	"../proxmox"
	"encoding/json"
	"errors"
	"os"
)

func init() {
	testActions["configlxc_createvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigLxcFromJson(os.Stdin)
		failOnError(err)

		vmr.SetNode(options.Args[1])
		config.CreateVm(vmr, client)
		return nil, nil
	}

	testActions["configlxc_clonevm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigLxcFromJson(os.Stdin)
		failOnError(err)

		DebugMsg("Looking for template: " + options.VMname)
		sourceVmr, err := client.GetVmRefByName(options.VMname)

		failOnError(err)
		if sourceVmr == nil {
			return nil, errors.New("ERROR: can't find template")
		}

		vmr.SetNode(options.Args[2])

		config.CloneVm(sourceVmr, vmr, client)
		return nil, err
	}

	testActions["configlxc_updateconfig"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigLxcFromJson(os.Stdin)
		failOnError(err)

		vminfo, err := client.GetVmInfo(vmr)
		failOnError(err)

		vmr.SetNode(vminfo["node"].(string))
		vmr.SetVmType(vminfo["type"].(string))
		config.UpdateConfig(vmr, client)
		return nil, err
	}

	testActions["configlxc_newconfiglxcfromjson"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.NewConfigLxcFromJson(os.Stdin)
	}

	testActions["configlxc_newconfiglxcfromapi"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return proxmox.NewConfigLxcFromApi(vmr, client)
	}

	testActions["configlxc_createlxcnetworksparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network is needed on stdin
		inputparams := proxmox.LxcDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as Net[0] as if it were built by
		// NewConfigLxcFromJson
		config := &proxmox.ConfigLxc{
			Net: proxmox.LxcDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.LxcDevice{}
		return premap, config.CreateLxcNetParams(options.VMid, premap)
	}

	testActions["configlxc_createlxcmpparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network interfaces is needed on stdin
		inputparams := proxmox.LxcDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as LxcDisks[0] as if it were built by
		// NewConfigLxcFromJson
		config := &proxmox.ConfigLxc{
			Mp: proxmox.LxcDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.LxcDevice{}
		return premap, config.CreateLxcMpParams(options.VMid, premap, false)
	}

}
