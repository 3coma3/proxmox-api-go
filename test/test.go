package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"bufio"
	"crypto/tls"
	// "encoding/json"
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

// this is so I don't have to type this awful type again and again when casting
func toMSI(i interface{}) map[string]interface{} {
	return i.(map[string]interface{})
}

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

	fmt.Print("\n")
}

// this is done repeatedly on most Client and ConfigQemu tests, abstracting here
func newClientAndVmr(options *TOptions) (client *proxmox.Client, vm *proxmox.Vm) {
	var err error

	DebugMsg("New Client with Login")

	tlsconf := &tls.Config{InsecureSkipVerify: true}
	if !options.APIinsecure {
		tlsconf = nil
	}

	if client, err = proxmox.NewClient(options.APIurl, nil, tlsconf); err != nil {
		log.Fatal(err)
	}

	askUserPass(options)

	if err = client.Login(options.APIuser, options.APIpass); err != nil {
		log.Fatal(err)
	}

	client.Set()

	// Auto VMId and Vm struct initialization
	if options.VMid <= 0 {
		if options.VMid, err = proxmox.GetNextVmId(0); err != nil {
			log.Fatal(err)
		}

	}

	vm = proxmox.NewVm(options.VMid)

	DebugMsg("vmid is " + strconv.Itoa(options.VMid))
	DebugMsg("vm is " + fmt.Sprintf("%+v", vm))

	return
}

// this is for those tests that don't use the integrated login of a Client
// creation or the interface of Client, but need to be logged to PVEAPI to
// run (mostly the Session tests)
func newSessionWithLogin(options *TOptions) (session *proxmox.Session) {
	var err error

	tlsconf := &tls.Config{InsecureSkipVerify: true}
	if !options.APIinsecure {
		tlsconf = nil
	}

	if session, err = proxmox.NewSession(options.APIurl, nil, tlsconf); err != nil {
		log.Fatal(err)
	}

	askUserPass(options)

	if err = session.Login(options.APIuser, options.APIpass); err != nil {
		log.Fatal(err)
	}

	return
}

// Run the test
func Run(options *TOptions) (err error) {
	// this is a "pseudo action", not subject to lookups in the actions map
	if options.Action == "listactions" {
		for action := range testActions {
			fmt.Println(action)
		}
		return
	}

	if test, exists := testActions[options.Action]; exists {
		proxmox.Debug = &Debug

		var response interface{}
		response, err = test(options)

		if response != nil {
			DebugMsg("The test returned a response:")
			if _, ok := response.(map[string]interface{}); ok {
				jsonPrettyPrint, _ := json.MarshalIndent(response, "", "  ")
				DebugMsg(string(jsonPrettyPrint))
			} else {
				fmt.Println(fmt.Sprintf("%v", response))
			}
		}

		return err
	}

	log.Fatal(errors.New("FATAL: action '" + options.Action + "' does not exist"))
	return
}

func DebugMsg(msg string) {
	if Debug {
		log.Println("DEBUG: " + msg)
	}
}
