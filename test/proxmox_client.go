package test

func init() {
	// factory
	testActions["client_newclient"] = errNotImplemented

	// tested in session_login
	testActions["client_login"] = errNotImplemented

	// TODO
	testActions["client_getjsonretryable"] = errNotImplemented
	testActions["client_waitforcompletion"] = errNotImplemented

	testActions["client_gettaskexitstatus"] = func(options *TOptions) (response interface{}, err error) {
		client, _ := newClientAndVmr(options)
		return client.GetTaskExitstatus(options.Args[1])
	}
}
