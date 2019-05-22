package test

import (
	"github.com/3coma3/proxmox-api-go/proxmox"
	"crypto/tls"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func init() {
	// factory
	testActions["session_newsession"] = errNotImplemented

	testActions["session_paramstobody"] = func(options *TOptions) (response interface{}, err error) {
		if config, err := proxmox.NewConfigQemuFromJson(os.Stdin); err == nil {

			params := map[string]interface{}{
				"vmid":        options.VMid,
				"name":        config.Name,
				"onboot":      config.Onboot,
				"ide2":        config.Iso + ",media=cdrom",
				"ostype":      config.Ostype,
				"sockets":     config.Sockets,
				"cores":       config.Cores,
				"cpu":         "host",
				"memory":      config.Memory,
				"description": config.Description,
			}

			// Create disks config.
			config.CreateDisksParams(options.VMid, params, false)

			// Create networks config.
			config.CreateNetParams(options.VMid, params)

			response = proxmox.ParamsToBody(params)
		}

		return
	}

	// to test this we could use the "manual workflow"
	testActions["session_responsejson"] = func(options *TOptions) (response interface{}, err error) {
		s := newSessionWithLogin(options)

		// let's use GET requests to list some items in PVE
		if resp, err := s.Get("/nodes", nil, nil); err == nil {
			return proxmox.ResponseJSON(resp)
		}

		return
	}

	// TODO
	testActions["session_typedresponse"] = errNotImplemented

	// to test this try first with valid tokens, then with invalid
	testActions["session_login"] = func(options *TOptions) (response interface{}, err error) {
		var session *proxmox.Session

		// create the session manually
		tlsconf := &tls.Config{InsecureSkipVerify: true}
		if !options.APIinsecure {
			tlsconf = nil
		}

		if session, err = proxmox.NewSession(options.APIurl, nil, tlsconf); err == nil {
			tryLogin := func(s string) error {
				DebugMsg("Attempting login with " + s + " tokens")
				options.APIuser, options.APIpass = "", ""
				askUserPass(options)
				return session.Login(strings.TrimSuffix(options.APIuser, "\n"), strings.TrimSuffix(options.APIpass, "\n"))
			}

			if tryLogin("VALID") != nil {
				err = errors.New("Attempt to login with VALID tokens was unsuccessful")
			} else if tryLogin("INVALID") == nil {
				err = errors.New("Attempt to login with INVALID tokens was successful")
			}
		}

		return
	}

	// simple factory
	testActions["session_newrequest"] = errNotImplemented

	// lowlevel and simple enough. Actually this should really be a private method,
	// it's only ever called from Session.Request()
	testActions["session_do"] = errNotImplemented

	// the functions below are already called from every other test / code, but the
	// explicit tests are put in place anyway

	// these two can be tested with GET queries to avoid effects
	// the effectful calls are tested below with their callers

	// from command line:
	// scripts/runtests.sh -insecure -debug session_request <endpoint> <parameters>
	// an optional third positional parameter after action and endpoint is
	// a comma delimited key=value string (no spaces) ie
	// param1=value,param2=value,...,paramN=value
	testActions["session_request"] = func(options *TOptions) (response interface{}, err error) {
		s := newSessionWithLogin(options)

		var params *url.Values
		if len(options.Args) > 2 {
			params = kvToParams(options.Args[2])
		}

		return s.Request("GET", options.Args[1], params, &s.Headers, nil)
	}

	// same command line semantics as with session_request
	testActions["session_requestjson"] = func(options *TOptions) (response interface{}, err error) {
		s := newSessionWithLogin(options)

		var params *url.Values
		if len(options.Args) > 2 {
			params = kvToParams(options.Args[2])
		}

		// this type is needed for json.Unmarshal to store a JSON value here
		respcontainer := new(map[string]interface{})

		_, err = s.RequestJSON("GET", options.Args[1], params, &s.Headers, nil, respcontainer)

		return respcontainer, err
	}

	// this is practically the same as the test for session_request
	testActions["session_get"] = func(options *TOptions) (response interface{}, err error) {
		s := newSessionWithLogin(options)

		var params *url.Values
		if len(options.Args) > 2 {
			params = kvToParams(options.Args[2])
		}

		return s.Get(options.Args[1], params, &s.Headers)
	}

	// this is practically the same as the test for session_requestjson
	testActions["session_getjson"] = func(options *TOptions) (response interface{}, err error) {
		s := newSessionWithLogin(options)

		var params *url.Values
		if len(options.Args) > 2 {
			params = kvToParams(options.Args[2])
		}

		// this type is needed for json.Unmarshal to store a JSON value here
		respcontainer := new(map[string]interface{})

		_, err = s.GetJSON(options.Args[1], params, &s.Headers, respcontainer)

		return respcontainer, err
	}

	// The more "side-effectful" requests need to cancel with a counterpart request,
	// ie a call to DELETE needs a bogus object to be created first, a creation
	// or change by POST or PUT needs to be deleted after.
	// This also implies that these tests cannot handle arbitrary endpoints from
	// user input, since POST/PUT semantics vary between endpoints, sometimes
	// POST/PUT means "use asynchronous/synchronous operation" sometimes they
	// mean create/change, sometimes one is not defined for the endpoint.

	// Ideally we should try to effect on resources that have the least impact.
	// The best candidate are resource pools, but they need an existing VM to
	// add to it. These tests will use the VMID parameter from command line and
	// expect the corresponding VM to exist.

	// The tests should follow the sequence:
	// 1) POST (creation)
	// 2) PUT (modification of the pool by adding and then removing a VM)
	// 3) DELETE (deletion)

	// this needs an argument with the pool name
	testActions["session_post"] = func(options *TOptions) (response interface{}, err error) {
		var (
			resp         interface{}
			found        bool
			testpoolname = options.Args[1]
			s            = newSessionWithLogin(options)
		)

		DebugMsg("Attempting to POST a new pool \"" + testpoolname + "\".")
		reqbody := proxmox.ParamsToBody(map[string]interface{}{"poolid": testpoolname, "comment": "automatically created by test code"})
		if _, err = s.Post("/pools", nil, nil, &reqbody); err == nil {
			// present information about the created pool
			_, _ = s.GetJSON("/pools", nil, &s.Headers, &resp)

			if _, err = s.GetJSON("/pools", nil, &s.Headers, resp); err == nil {
				for _, pool := range toMSI(resp)["data"].([]interface{}) {
					if toMSI(pool)["poolid"].(string) == testpoolname {
						response = pool
						found = true
						break
					}
				}
			}

			if !found {
				err = errors.New("Couldn't find that the pool was created")
			} else {
				DebugMsg("Found the pool \"" + testpoolname + "\" just created.")
			}
		}

		return
	}

	// the only difference with the session_post test is the auto
	// deserialization, this was already tested in RequestJSON
	testActions["session_postjson"] = errNotImplemented

	// this needs an argument with the pool name and existing -vmid to add
	testActions["session_put"] = func(options *TOptions) (response interface{}, err error) {
		var (
			resp         interface{}
			found        bool
			testpoolname = options.Args[1]
			s            = newSessionWithLogin(options)
		)

		_, vm := newClientAndVmr(options)
		if err = vm.Check(); err != nil {
			return
		}

		if _, err = s.GetJSON("/pools", nil, &s.Headers, &resp); err != nil {
			return
		}

		for _, pool := range toMSI(resp)["data"].([]interface{}) {
			if toMSI(pool)["poolid"].(string) == testpoolname {
				found = true
				break
			}
		}

		if !found {
			err = errors.New("Couldn't find that the pool exists")
			return
		}
		DebugMsg("Found the pool \"" + testpoolname + "\"")

		DebugMsg("Attempting to add the VM " + strconv.Itoa(options.VMid) + " to the pool " + testpoolname)
		reqbody := proxmox.ParamsToBody(map[string]interface{}{"vms": strconv.Itoa(options.VMid)})
		if _, err = s.Put("/pools/"+testpoolname, nil, nil, &reqbody); err != nil {
			return
		}

		// present information about the pool modification
		if _, err = s.GetJSON("/pools/"+testpoolname, nil, &s.Headers, &resp); err == nil {
			found = false
			for _, member := range toMSI(toMSI(resp)["data"])["members"].([]interface{}) {
				if toMSI(member)["id"].(string) == vm.Type()+"/"+strconv.Itoa(options.VMid) {
					found = true
					break
				}
			}

			if !found {
				err = errors.New("Couldn't verify the VM addition")
				return
			}
		}
		DebugMsg("Found the VM " + strconv.Itoa(options.VMid) + " in the pool " + testpoolname)

		DebugMsg("Attempting to remove the VM " + strconv.Itoa(options.VMid) + " from the pool " + testpoolname)
		reqbody = proxmox.ParamsToBody(map[string]interface{}{"vms": strconv.Itoa(options.VMid), "delete": true})
		if _, err = s.Put("/pools/"+testpoolname, nil, nil, &reqbody); err != nil {
			return
		}

		// present information about the pool modification
		if _, err = s.GetJSON("/pools/"+testpoolname, nil, &s.Headers, &resp); err == nil {
			found = false
			for _, member := range toMSI(toMSI(resp)["data"])["members"].([]interface{}) {
				if toMSI(member)["id"].(string) == vm.Type()+"/"+strconv.Itoa(options.VMid) {
					found = true
					break
				}
			}

			if found {
				err = errors.New("The VM " + strconv.Itoa(options.VMid) + " could not be removed by PUT")
				return
			}
		}

		DebugMsg("Successfully removed the VM " + strconv.Itoa(options.VMid) + " from the pool " + testpoolname)

		return "test OK", nil
	}

	// this expects an existing pool to be specified in command line
	testActions["session_delete"] = func(options *TOptions) (response interface{}, err error) {
		var (
			resp         interface{}
			found        bool
			testpoolname = options.Args[1]
			s            = newSessionWithLogin(options)
		)

		if _, err = s.GetJSON("/pools", nil, &s.Headers, &resp); err != nil {
			return
		}

		for _, pool := range toMSI(resp)["data"].([]interface{}) {
			if toMSI(pool)["poolid"].(string) == testpoolname {
				found = true
				break
			}
		}

		if !found {
			err = errors.New("Couldn't find that the pool exists")
			return
		}
		DebugMsg("Found the pool \"" + testpoolname + "\"")

		DebugMsg("Attempting to remove the pool \"" + testpoolname + "\".")
		return s.Delete("/pools/"+testpoolname, nil, &s.Headers)
	}

	// this one doesn't have callers, like GET and DELETE. But HEAD also isn't
	// a PVEAPI REST operation, so not testing
	testActions["session_head"] = errNotImplemented
}
