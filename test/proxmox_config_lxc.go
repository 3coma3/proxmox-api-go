package test

import (
	"../proxmox"
	"encoding/json"
	"errors"
	"os"
)

func init() {
	testActions["configqemu_createvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigQemuFromJson(os.Stdin)
		failOnError(err)

		vmr.SetNode(options.Args[1])
		config.CreateVm(vmr, client)
		return nil, nil
	}

	// simple method
	testActions["configqemu_hascloudinit"] = errNotImplemented

	testActions["configqemu_clonevm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigQemuFromJson(os.Stdin)
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

	testActions["configqemu_updateconfig"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		config, err := proxmox.NewConfigQemuFromJson(os.Stdin)
		failOnError(err)

		vminfo, err := client.GetVmInfo(vmr)
		failOnError(err)

		vmr.SetNode(vminfo["node"].(string))
		vmr.SetVmType(vminfo["type"].(string))
		config.UpdateConfig(vmr, client)
		return nil, err
	}

	testActions["configqemu_newconfigqemufromjson"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.NewConfigQemuFromJson(os.Stdin)
	}

	testActions["configqemu_newconfigqemufromapi"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return proxmox.NewConfigQemuFromApi(vmr, client)
	}

	testActions["configqemu_waitforshutdown"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)

		// remember to use this to shutdown asynchronously
		_, err = testActions["client_monitorcmd"](options)
		failOnError(err)
		return nil, proxmox.WaitForShutdown(vmr, client)
	}

	testActions["configqemu_sshforwardusernet"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return proxmox.SshForwardUsernet(vmr, client)
	}

	testActions["configqemu_removesshforwardusernet"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return nil, proxmox.RemoveSshForwardUsernet(vmr, client)
	}

	testActions["configqemu_maxvmid"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return proxmox.MaxVmId(client)
	}

	testActions["configqemu_sendkeysstring"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return nil, proxmox.SendKeysString(vmr, client, options.Args[1])
	}

	testActions["configqemu_createqemunetworksparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network is needed on stdin
		inputparams := proxmox.QemuDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as QemuNetworks[0] as if it were built by
		// NewConfigQemuFromJson
		config := &proxmox.ConfigQemu{
			QemuNetworks: proxmox.QemuDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.QemuDevice{}
		return premap, config.CreateQemuNetworksParams(options.VMid, premap)
	}

	testActions["configqemu_createqemudisksparams"] = func(options *TOptions) (response interface{}, err error) {
		// only the json for the network interfaces is needed on stdin
		inputparams := proxmox.QemuDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as QemuDisks[0] as if it were built by
		// NewConfigQemuFromJson
		config := &proxmox.ConfigQemu{
			QemuDisks: proxmox.QemuDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.QemuDevice{}
		return premap, config.CreateQemuDisksParams(options.VMid, premap, false)
	}

}
