package test

import (
	"../proxmox"
	"bufio"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// TOptions - test configuration and parameters
type TOptions struct {
	Action      string
	VMid        int
	VMname      string
	Args        []string
	APIurl      string
	APIuser     string
	APIpass     string
	APIinsecure bool
}

type testAction func(*TOptions) (interface{}, error)

var (
	Debug = false

	testActions = map[string]testAction{}

	errNotImplemented = func(o *TOptions) (interface{}, error) {
		return nil, errors.New("ERROR: the test '" + o.Action + "' is not implemented yet")
	}
)

// parses a string of the form "key1=value1,...keyN=valueN" to a value of
// type url.Values, to send as a query string
// this is similar to what ParseConf / ParseSubConf at util.go do
// The difference is mostly in the return type and fixed delimiters
func kvToParams(kvstring string) (params *url.Values) {
	params = &url.Values{}
	kvs := strings.Split(kvstring, ",")
	for i := 0; i < len(kvs); i++ {
		kv := strings.Split(kvs[i], "=")
		params.Add(kv[0], kv[1])
	}
	return
}

// from https://stackoverflow.com/a/31571984
// used in askUserPass for the password input
func terminalEcho(show bool) {
	var termios = &syscall.Termios{}
	var fd = os.Stdout.Fd()

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd,
		syscall.TCGETS, uintptr(unsafe.Pointer(termios))); err != 0 {
		return
	}

	if show {
		termios.Lflag |= syscall.ECHO
	} else {
		termios.Lflag &^= syscall.ECHO
	}

	if _, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd,
		uintptr(syscall.TCSETS),
		uintptr(unsafe.Pointer(termios))); err != 0 {
		return
	}
}

func askUserPass(options *TOptions) {
	reader := bufio.NewReader(os.Stdin)

	if options.APIuser == "" {
		fmt.Print("Enter username and realm: ")
		user, _ := reader.ReadString('\n')
		options.APIuser = strings.TrimSuffix(user, "\n")
	}

	if options.APIpass == "" {
		fmt.Print("Enter password: ")
		terminalEcho(false)
		pass, _ := reader.ReadString('\n')
		terminalEcho(true)
		options.APIpass = strings.TrimSuffix(pass, "\n")
	}
}

// this is done repeatedly on most Client and ConfigQemu tests, abstracting here
func newClientAndVmr(options *TOptions) (client *proxmox.Client, v *proxmox.Vm) {
	DebugMsg("New Client with Login")

	tlsconf := &tls.Config{InsecureSkipVerify: true}
	if !options.APIinsecure {
		tlsconf = nil
	}

	client, err := proxmox.NewClient(options.APIurl, nil, tlsconf)
	failOnError(err)

	askUserPass(options)

	failOnError(client.Login(options.APIuser, options.APIpass))

	client.Set()

	// Auto VMId and Vm struct initialization
	if options.VMid <= 0 {
		options.VMid, err = proxmox.GetNextVmId(0)
		failOnError(err)
	}

	v = proxmox.NewVm(options.VMid)

	DebugMsg("vmid is " + strconv.Itoa(options.VMid))
	DebugMsg("v is " + fmt.Sprintf("%+v", v))

	return
}

// this is for those tests that don't use the integrated login of a Client
// creation or the interface of Client, but need to be logged to PVEAPI to
// run (mostly the Session tests)
func newSessionWithLogin(options *TOptions) (session *proxmox.Session) {
	tlsconf := &tls.Config{InsecureSkipVerify: true}
	if !options.APIinsecure {
		tlsconf = nil
	}

	session, err := proxmox.NewSession(options.APIurl, nil, tlsconf)
	failOnError(err)

	askUserPass(options)

	failOnError(session.Login(options.APIuser, options.APIpass))
	return
}

// Run the test
func Run(options *TOptions) bool {
	// this is a "pseudo action", as it is not subject to lookups in the actions
	// map, it is hardcoded here.
	if options.Action == "listactions" {
		for action := range testActions {
			fmt.Println(action)
		}
		return true
	}

	var (
		response interface{}
		err      error
	)

	// lookup
	if test, ok := testActions[options.Action]; ok {
		// used by the proxmox pkg
		proxmox.Debug = &Debug

		response, err = test(options)
		failOnError(err)

		if response != nil {
			response, _ := json.MarshalIndent(response, "", "  ")
			DebugMsg("response is " + string(response))
		} else {
			DebugMsg("response is nil")
		}

		return true
	}
	return failOnError(errors.New("FATAL: action '" + options.Action + "' does not exist"))
}

func DebugMsg(msg string) {
	if Debug {
		log.Println("DEBUG: " + msg)
	}
}

func failOnError(err error) bool {
	if err != nil {
		log.Fatal(err)
		return false
	}

	return true
}
