package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"encoding/json"
	"os"
)

func init() {
	// factories getters and setters
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
		if err = json.NewDecoder(os.Stdin).Decode(&cloneParams); err == nil {
			DebugMsg("Looking for template: " + options.VMname)
			if sourceVm, err := proxmox.FindVm(options.VMname); err == nil && sourceVm != nil {
				return sourceVm.Clone(vm.Id(), cloneParams)
			}
		}

		return
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
		if _, err = testActions["vm_monitorcmd"](options); err != nil {
			return
		}

		return nil, v.WaitForShutdown()
	}

	testActions["vm_migrate"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		migrateParams := map[string]interface{}{}
		if err = json.NewDecoder(os.Stdin).Decode(&migrateParams); err != nil {
			return
		}

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
		if err = json.NewDecoder(os.Stdin).Decode(&snapParams); err != nil {
			return
		}

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
		if err = json.NewDecoder(os.Stdin).Decode(&bkpParams); err != nil {
			return
		}

		return vm.CreateBackup(bkpParams)
	}

	testActions["vm_movedisk"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)

		moveParams := map[string]interface{}{}
		if err = json.NewDecoder(os.Stdin).Decode(&moveParams); err != nil {
			return
		}

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

	testActions["vm_getagentnetworkinterfaces"] = func(options *TOptions) (response interface{}, err error) {
		_, vm := newClientAndVmr(options)
		return vm.GetAgentNetworkInterfaces()
	}
}
