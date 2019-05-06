package main

import (
	"flag"
	"fmt"
	"github.com/3coma3/proxmox-api-go/test"
	"os"
	"strconv"
)

func main() {
	defer func() {
		// exit with an error in case of errors
		os.Exit(1)
	}()

	var (
		err error

		options test.TOptions = test.TOptions{
			Action:      "",
			VMid:        0,
			VMname:      "",
			APIurl:      os.Getenv("PM_API_URL"),
			APIuser:     os.Getenv("PM_USER"),
			APIpass:     os.Getenv("PM_PASS"),
			APIinsecure: false,
		}

		fvmid     = flag.Int("vmid", options.VMid, "custom vmid (instead of auto)")
		fdebug    = flag.Bool("debug", false, "debug mode")
		finsecure = flag.Bool("insecure", options.APIinsecure, "TLS insecure mode")
	)

	flag.Parse()

	if len(flag.Args()) == 0 {
		fmt.Printf("Usage: %s [flags] [action] [vmid|vname] [host]\n", os.Args[0])
		fmt.Printf("\nFlags: \n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// used by test.DebugMsg
	test.Debug = *fdebug

	test.DebugMsg("-insecure is " + strconv.FormatBool(*finsecure))
	test.DebugMsg("-debug is " + strconv.FormatBool(*fdebug))
	test.DebugMsg("-fvmid is " + strconv.Itoa(*fvmid))
	for i, v := range flag.Args() {
		test.DebugMsg("flag.Args()[" + strconv.Itoa(i) + "] is " + v)
	}

	options.Action = flag.Args()[0]

	// Try to get VMid from the second (non-flag) positional parameter
	// If that fails (argument is not numeric) or -vmid was specified, just
	// set the field to whatever *fvmid is and assume the argument is a VMname
	// This implies that -vmid wins when both it and the positional parameter
	// are present to set the VMid
	// The "extra parameter" could be also set here (see below)
	if len(flag.Args()) > 1 {
		if options.VMid, err = strconv.Atoi(flag.Args()[1]); *fvmid > 0 || err != nil {
			options.VMid = *fvmid
			options.VMname = flag.Args()[1]
		}
	} else {
		options.VMid = *fvmid
	}

	// for other parameters than vm identifications I refrain to do more checks
	// to avoid spending time in a CLI until we decide to go with this codebase
	// copying the arguments array to the options
	// a good solution might be something like https://github.com/mitchellh/mapstructure
	// or directly https://github.com/urfave/cli
	options.Args = flag.Args()

	options.APIinsecure = *finsecure

	// Other validations could be done here, like having extra positional
	// parameters beyond 3 (action, vmid/vname, node) or having an action that
	// actually cares about what has been passed
	// Ignoring these conditions is simpler and won't break things, though

	// the failOnError, fatal, and bools will be later switched to proper
	// error instance bubbling / wrapping back to main
	{
		test.DebugMsg("Running the test: " + options.Action)
		err := test.Run(&options)
		test.DebugMsg("The test " + options.Action + " has returned: " + strconv.FormatBool(err))
		if !err {
			panic("Test failed")
		}
	}
}
