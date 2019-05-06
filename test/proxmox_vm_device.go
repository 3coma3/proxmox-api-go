package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
)

func init() {
	// don't need a test
	testActions["util_inarray"] = errNotImplemented
	testActions["util_itob"] = errNotImplemented

	// for the multiple return value to fit in the response it will be packaged
	// in a map
	testActions["util_parsesubconf"] = func(options *TOptions) (interface{}, error) {
		var response = map[string]interface{}{}

		key, value := proxmox.ParseSubConf(options.Args[1], options.Args[2])
		response[key] = value

		return response, nil
	}

	testActions["util_parseconf"] = func(options *TOptions) (response interface{}, err error) {
		return proxmox.ParseConf(options.Args[1], options.Args[2], options.Args[3]), nil
	}
}
