package test

import (
	"encoding/json"
	"errors"
	"github.com/3coma3/proxmox-api-go/proxmox"
	"log"
	"os"
)

func init() {
	// lean factories
	// getters and setters
	testActions["vm_newm"] = errNotImplemented
	testActions["vm_id"] = errNotImplemented
	testActions["vm_node"] = errNotImplemented
	testActions["vm_setnode"] = errNotImplemented
	testActions["vm_setype"] = errNotImplemented

	testActions["vm_check"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return nil, vm.Check()
	}

	testActions["vm_getvmlist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetVmList()
	}

	testActions["vm_getinfo"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetInfo()
	}

	testActions["vm_findvm"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.FindVm(options.VMname)
	}

	testActions["vm_getmaxvmid"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetMaxVmId()
	}

	testActions["vm_getnextvmid"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetNextVmId(options.VMid)
	}

	// moved to config*_createvm, as the action starts there
	testActions["vm_create"] = errNotImplemented

	testActions["vm_createtemplate"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.CreateTemplate()
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

	testActions["vm_delete"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Delete()
	}

	testActions["vm_getconfig"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetConfig()
	}

	// moved to config*_updateconfig, as the action starts there
	testActions["vm_setconfig"] = errNotImplemented

	testActions["vm_getstatus"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetStatus()
	}

	testActions["vm_setstatus"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.SetStatus(options.Args[1])
	}

	testActions["vm_start"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Start()
	}

	testActions["vm_suspend"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Suspend()
	}

	testActions["vm_resume"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Resume()
	}

	testActions["vm_reset"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Reset()
	}

	testActions["vm_stop"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Stop()
	}

	testActions["vm_shutdown"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Shutdown()
	}

	testActions["vm_waitforshutdown"] = func(options *TOptions) (response interface{}, err error) {
		_, v := newClientAndVmr(options)

		// remember to use this to shutdown asynchronously
		_, err = testActions["vm_monitorcmd"](options)
		failOnError(err)
		return nil, v.WaitForShutdown()
	}

	testActions["vm_migrate"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		migrateParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&migrateParams))

		migrateParams["target"] = options.Args[1]

		return vm.Migrate(migrateParams)
	}

	testActions["vm_getsnapshotlist"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetSnapshotList()
	}

	testActions["vm_createsnapshot"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		snapParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&snapParams))

		snapParams["snapname"] = options.Args[1]

		return vm.CreateSnapshot(snapParams)
	}

	testActions["vm_deletesnapshot"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.DeleteSnapshot(options.Args[1])
	}

	testActions["vm_rollback"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.Rollback(options.Args[1])
	}

	testActions["vm_createbackup"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		bkpParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&bkpParams))

		return vm.CreateBackup(bkpParams)
	}

	// this test sheds some light on the reason for the multiple mappings and
	// translations between them: there is a Json for user input, another for
	// PVE, and another format for the methods that create disks. Validations
	// must be done on maps so WHERE these validations are done could be key
	// in simplifying the scheme (try to avoid to generate configuration
	// strings too early for example, so we have to stop-by and create a map
	// to  manipulate and do checks, then translate again... so on)
	testActions["node_createvolume"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)

		// only the json for the disks is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&inputparams))

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

				// this map is specially prepared for the disk creation
				diskParams := map[string]interface{}{
					"vmid": options.VMid,
					"size": deviceConfMap["size"],
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
				return nil, proxmox.NewNode(options.Args[1]).CreateVolume(fullDiskName, diskParams)
			}
		}

		return
	}

	testActions["vm_movedisk"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		moveParams := map[string]interface{}{}
		failOnError(json.NewDecoder(os.Stdin).Decode(&moveParams))
		return vm.MoveDisk(moveParams)
	}

	testActions["vm_resizedisk"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.ResizeDisk(options.Args[1], options.Args[2])
	}

	testActions["vm_getspiceproxy"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetSpiceProxy()
	}

	testActions["vm_monitorcmd"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.MonitorCmd(options.Args[1])
	}

	testActions["vm_sendkeysstring"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return nil, vm.SendKeysString(options.Args[1])
	}

	testActions["vm_sshforwardusernet"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.SshForwardUsernet()
	}

	testActions["vm_removesshforwardusernet"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return nil, vm.RemoveSshForwardUsernet()
	}

	testActions["vm_getagentnetworkinterfaces"] = errNotImplemented
}
