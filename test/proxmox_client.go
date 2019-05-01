package test

import (
	"../proxmox"
	"encoding/json"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	// getters and setters
	testActions["client_setnode"] = errNotImplemented
	testActions["client_setvmtype"] = errNotImplemented
	testActions["client_vmid"] = errNotImplemented
	testActions["client_node"] = errNotImplemented

	// lean factories
	testActions["client_newvmref"] = errNotImplemented
	testActions["client_newclient"] = errNotImplemented

	// TODO
	testActions["client_getjsonretryable"] = errNotImplemented

	testActions["client_getnodelist"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetNodeList()
	}

	testActions["client_getvmlist"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetVmList()
	}

	testActions["client_checkvmref"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return nil, client.CheckVmRef(vmr)
	}

	testActions["client_getvminfo"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.GetVmInfo(vmr)
	}

	testActions["client_getvmrefbyname"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetVmRefByName(options.VMname)
	}

	testActions["client_getvmstate"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.GetVmState(vmr)
	}

	testActions["client_getvmconfig"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.GetVmConfig(vmr)
	}

	testActions["client_getvmspiceproxy"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.GetVmSpiceProxy(vmr)
	}

	testActions["client_createtemplate"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return nil, client.CreateTemplate(vmr)
	}

	testActions["client_monitorcmd"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.MonitorCmd(vmr, options.Args[1])
	}

	// not very testable as it depends on a response for a previously dispatched
	// request, and it's already used by the VM creation code that is tested
	testActions["client_waitforcompletion"] = errNotImplemented

	testActions["client_gettaskexitstatus"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetTaskExitstatus(options.Args[1])
	}

	testActions["client_statuschangevm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.StatusChangeVm(vmr, options.Args[1])
	}

	testActions["client_startvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.StartVm(vmr)
	}

	testActions["client_stopvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.StopVm(vmr)
	}

	testActions["client_shutdownvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.ShutdownVm(vmr)
	}

	testActions["client_resetvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.ResetVm(vmr)
	}

	testActions["client_suspendvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.SuspendVm(vmr)
	}

	testActions["client_resumevm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.ResumeVm(vmr)
	}

	testActions["client_deletevm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.DeleteVm(vmr)
	}

	// moved to configqemu_createvm, as it would involve replicating here the
	// target method anyway, so that test suffices
	testActions["client_createqemuvm"] = errNotImplemented

	// moved to configqemu_clone, as it would involve replicating here the
	// target method anyway, so that test suffices
	testActions["client_cloneqemuvm"] = errNotImplemented

	testActions["client_rollbackqemuvm"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		return client.RollbackQemuVm(vmr, options.Args[1])
	}

	// moved to configqemu_updateconfig, as it would involve replicating here the
	// target method anyway, so that test suffices
	testActions["client_setvmconfig"] = errNotImplemented

	testActions["client_getnextid"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetNextID(options.VMid)
	}

	testActions["client_resizeqemudisk"] = func(options *TOptions) (response interface{}, err error) {
		client, vmr := newClientAndVmr(options)
		vmr.SetNode(options.Args[1])
		vmr.SetVmType("qemu")
		moreSizeGB, err := strconv.Atoi(options.Args[3])
		failOnError(err)
		return client.ResizeQemuDisk(vmr, options.Args[2], moreSizeGB)

	}

	// this test sheds some light on the reason for the multiple mappings and
	// translations between them: there is a Json for user input, another for
	// PVE, and another format for the methods that create disks. Validations
	// must be done on maps so WHERE these validations are done could be key
	// in simplifying the scheme (try to avoid to generate configuration
	// strings too early for example, so we have to stop-by and create a map
	// to  manipulate and do checks, then translate again... so on)
	testActions["client_createvmdisk"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)

		// only the json for the disks is needed on stdin
		inputparams := proxmox.QemuDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as QemuDisks[0] as if it were built by NewConfigQemuFromJson
		config := &proxmox.ConfigQemu{
			QemuDisks: proxmox.QemuDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.QemuDevice{}
		config.CreateQemuDisksParams(options.VMid, premap, false)

		// separate the name and the configuration string for each premap entry
		// we won't need to filter device names looking for virtio,ide,etc as we
		// are testing and we know we will get a correct configuration
		for _, deviceConf := range premap {

			// build another map[string]interface{} out of the config string
			// "," separates a kv pair, "=" separates k from v
			deviceConfMap := proxmox.ParseConf(deviceConf.(string), ",", "=")

			// filter out `media=cdrom`.
			if media, containsFile := deviceConfMap["media"]; containsFile && media == "disk" {

				fullDiskName := deviceConfMap["file"].(string)

				// this step is done in DeleteVMDisks, but not in CreateVMDisk
				storageAndVolumeName := strings.Split(fullDiskName, ":")
				storageName, volumeName := storageAndVolumeName[0], storageAndVolumeName[1]

				// when disk type is dir, volumeName is `file=local:100/vm-100-disk-0.raw`
				match := regexp.MustCompile(`\d+/(?P<filename>\S+.\S+)`).FindStringSubmatch(volumeName)
				if len(match) == 2 {
					volumeName = match[1]
				}

				// this map is specially prepared for the disk creation
				diskParams := map[string]interface{}{
					"vmid":     options.VMid,
					"filename": volumeName,
					"size":     deviceConfMap["size"],
				}

				// this is a neat reference on all the mappings the code has to
				// do between user input, ConfigQemu/QemuDevice, deviceParam
				// (premap),  deviceConfMap and finally "diskParams" ...
				log.Println(inputparams)
				log.Println(premap)
				log.Println(deviceConf)
				log.Println(deviceConfMap)
				log.Println(diskParams)

				// anyway this is what the method needs to function, and it will
				// create the disk. The fixed parameters are mostly so it
				// doesn't have to parse again any map, it will use them for
				// information and checking the volume isn't there already
				// after creating the disk the function fails, finding out why
				// is what is left for this test to complete
				return nil, client.CreateVMDisk(options.Args[1], storageName, fullDiskName, diskParams)
			}
		}

		return
	}

	// This test will always fail, the target method uses an incorrect REST verb
	// (POST) while it should use DELETE
	testActions["client_deletevmdisks"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return nil, client.DeleteVMDisks(options.Args[1], strings.Split(options.Args[2], ","))
	}
}
