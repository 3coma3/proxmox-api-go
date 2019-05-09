package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"encoding/json"
	"log"
	"os"
)

func init() {
	// factory
	testActions["node_newnode"] = errNotImplemented

	testActions["node_getnodelist"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.GetNodeList()
	}

	testActions["node_findnode"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.FindNode(options.Args[1])
	}

	testActions["node_check"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return nil, proxmox.NewNode(options.Args[1]).Check()
	}

	testActions["node_getinfo"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return proxmox.NewNode(options.Args[1]).GetInfo()
	}

	testActions["node_createvolume"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)

		// only the json for the disks is needed on stdin
		inputparams := proxmox.VmDevice{}

		// put whatever json is on stdin into a map[string]interface{}
		if err = json.NewDecoder(os.Stdin).Decode(&inputparams); err != nil {
			return
		}

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
				// after creating the disk the function fails
				// TODO: investigate the failure
				err = proxmox.NewNode(options.Args[1]).CreateVolume(fullDiskName, diskParams)
			}
		}

		return
	}

	testActions["node_deletevolume"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)
		return nil, proxmox.NewNode(options.Args[1]).DeleteVolume(options.Args[2])
	}

	testActions["node_getstorageandvolumename"] = func(options *TOptions) (response interface{}, err error) {
		_, _ = newClientAndVmr(options)

		storageName, volumeName := proxmox.GetStorageAndVolumeName(options.Args[0], options.Args[1])

		response = map[string]interface{}{
			"storageName": storageName,
			"volumeName":  volumeName,
		}

		return
	}
}
