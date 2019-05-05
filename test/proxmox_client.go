package test

import (
	"../proxmox"
	"encoding/json"
	"errors"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	// getters and setters
	testActions["vm_setnode"] = errNotImplemented
	testActions["vm_setype"] = errNotImplemented
	testActions["vm_vmid"] = errNotImplemented
	testActions["vm_node"] = errNotImplemented

	// lean factories
	testActions["vm_newm"] = errNotImplemented
	testActions["client_newclient"] = errNotImplemented

	// TODO
	testActions["client_getjsonretryable"] = errNotImplemented

	testActions["getnodelist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetNodeList()
	}

	testActions["getvmlist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetVmList()
	}

	testActions["vm_check"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return nil, vm.Check()
	}

	testActions["vm_getvminfo"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetInfo()
	}

	testActions["client_getvmbyname"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.FindVm(options.VMname)
	}

	testActions["vm_getstatus"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetStatus()
	}

	testActions["vm_getconfig"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetConfig()
	}

	testActions["vm_getspiceproxy"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetSpiceProxy()
	}

	testActions["vm_createtemplate"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return nil, vm.CreateTemplate()
	}

	testActions["client_monitorcmd"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.MonitorCmd(options.Args[1])
	}

	// not very testable as it depends on a response for a previously dispatched
	// request, and it's already used by the VM creation code that is tested
	testActions["client_waitforcompletion"] = errNotImplemented

	testActions["client_gettaskexitstatus"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetTaskExitstatus(options.Args[1])
	}

	testActions["vm_changestatus"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.SetStatus(options.Args[1])
	}

	testActions["vm_start"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Start()
	}

	testActions["vm_stop"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Stop()
	}

	testActions["vm_shutdown"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Shutdown()
	}

	testActions["vm_reset"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Reset()
	}

	testActions["vm_suspend"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Suspend()
	}

	testActions["vm_resume"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Resume()
	}

	testActions["vm_delete"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Delete()
	}

	// moved to configqemu_createvm, as the action starts there
	testActions["vm_create"] = errNotImplemented

	testActions["vm_rollback"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Rollback(options.Args[1])
	}

	testActions["vm_clone"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		cloneParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&cloneParams))

		DebugMsg("Looking for template: " + options.VMname)
		sourceVm, err := proxmox.FindVm(options.VMname)

		failOnError(err)
		if sourceVm == nil {
			return nil, errors.New("ERROR: can't find template")
		}

		return sourceVm.Clone(vm.Id(), cloneParams)
	}

	testActions["vm_migrate"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		migrateParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&migrateParams))

		migrateParams["target"] = options.Args[1]

		return vm.Migrate(migrateParams)
	}

	// moved to configqemu_updateconfig, as the action starts there
	testActions["vm_setconfig"] = errNotImplemented

	testActions["client_getnextid"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetNextVmId(options.VMid)
	}

	testActions["vm_resizedisk"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		vm.SetNode(options.Args[1])
		vm.SetType("qemu")
		moreSizeGB, err := strconv.Atoi(options.Args[3])
		failOnError(err)
		return vm.ResizeDisk(options.Args[2], moreSizeGB)

	}

	// this test sheds some light on the reason for the multiple mappings and
	// translations between them: there is a Json for user input, another for
	// PVE, and another format for the methods that create disks. Validations
	// must be done on maps so WHERE these validations are done could be key
	// in simplifying the scheme (try to avoid to generate configuration
	// strings too early for example, so we have to stop-by and create a map
	// to  manipulate and do checks, then translate again... so on)
	testActions["vm_createdisk"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)

		// only the json for the disks is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

		// put the map as QemuDisks[0] as if it were built by NewConfigQemuFromJson
		config := &proxmox.ConfigQemu{
			Disk: proxmox.VmDevices{0: inputparams},
		}

		// so now this method can build the PVEAPI-compatible "premap"
		// this is a map of keys to config items, each config item will have
		// a device name and a configuration with two levels of subelements
		// this method rewrites heavily the input parameters
		premap := proxmox.VmDevice{}
		config.CreateDisksParams(options.VMid, premap, false)

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
				// do between user input, ConfigQemu/VmDevice, deviceParam
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
				return nil, proxmox.CreateDisk(options.Args[1], storageName, fullDiskName, diskParams)
			}
		}

		return
	}
}
