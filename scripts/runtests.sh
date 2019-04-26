#!/bin/bash

# DESCRIPTION AND USE

# this script can be used to call the proxmox-api-go binary with the PM_API_URL,
# PM_USER and PM_PASS variables set in the environment, to run a specific test
# while forwarding the command line to it.
#
# to use the script in this way, run it like this:
#
# runtests.sh <proxmox-api-go switches> <action> <action arguments>
#
# the proxmox-api-go switches are
#
# -debug: outputs extra information (recommended)
# -insecure: don't check TLS certs (recommended)
# -vmid: sets the VM ID parameter for the actions that require it
#
# <action> can be any action defined in the testActions map in the go testing
# code, to see all the actions defined you can pass "listactions" instead of an
# action name
#
# the action arguments depend on the specific action to test

# alternatively, this script can attempt to run every test defined following
# a sequence that combines some tests to work together, like creating a VM,
# then manipulating it, getting information about the VM and then delete it
#
# to use the script in this way, run it without arguments:
#
# runtests.sh


# SHELL DIRECTIVES

# this parser directive must be always at the top
# enable extended globs for pattern matching
shopt -s extglob


# GLOBAL DATA
declare scriptdir="$(dirname "$0")"

# list of test setups and their results
declare -a sequence
declare -A results

# these arrays are columns, each row is a VM handled by the tests
declare -a vmids vmnames vmconfigs

# for node selection
declare -a nodes
declare selectednode

# for POST/PUT/DELETE tests
declare testpoolname='testpool'

# for gettaskexitstatus
declare UPID


# CONFIGURATION - modify these values as needed
test_binary="$scriptdir/../proxmox-api-go"
test_default_flags='-debug -insecure'
setup_prefix='testsetup_'
export PM_API_URL='https://10.40.0.147:8006/api2/json' PM_USER='root@pam' PM_PASS


# HELPER FUNCTIONS

# outputs JSON configuration for Qemu VM disks
# parameters: $1 - disk name, $2 - disk size in GB, default 2GB
diskjson() {
  local filename="$1" size="${2:-2}"

    cat<<EOF
{
  "filename": "$filename",
  "storage": "local-lvm",
  "storage_type": "lvmthin",
  "type": "virtio",
  "cache": "none",
  "format": "raw",
  "size": "${size}G"
}
EOF
}

# outputs a random MAC address
# parameters: $1 - byte delimiter, default ':'
newmac() {
  local delimiter="${1:-:}"
  local h2ndbit="2367ABEF"
  local hexchars="0123456789ABCDEF"
  for i in {1..12} ; do
  # the first byte must have the lower nibble with the second LSB on 1
  # to point a Locally Administered Address
  if (( i == 2 )); then
      echo -n ${h2ndbit:$(( $RANDOM % ${#h2ndbit} )):1}
    else
      echo -n ${hexchars:$(( $RANDOM % ${#hexchars} )):1}
    fi
  done | sed -re 's/(..)/\1'$delimiter'/g' -e 's/(.*).$/\1/g'
}

# outputs JSON configuration for Qemu VMs
# parameters: $1 - VM name, default 'testvm'
vmjson() {
    local vmname="${1:-testvm}"
    cat<<EOF
{
  "name": "$vmname",
  "onboot": false,
  "memory": 2048,
  "ostype": "l26",
  "cores": 1,
  "sockets": 1,
  "iso": "local:iso/uccorelinux.iso",
  "disk": {
    "0": {
      "type": "virtio",
      "storage": "local-lvm",
      "storage_type": "lvmthin",
      "size": "2G",
      "cache": "none",
      "format": "raw"
    }
  },
  "network": {
    "0": {
      "model": "virtio",
      "bridge": "vmbr0",
      "macaddr": "$(newmac)" 
    }
  }
}
EOF
}


# MAIN CODE

# this function allows to express the testing sequence as declaratively and
# cleanly as possible, the downside is the logic at the setup handlers is a bit
# more convoluted at some points, but I still think it's well worth it
prepare_test_sequence() {
    local action

    echo -e "\nPreparing the test sequence\n"

    # start with the util tests
    sequence+=(util_parseconf)
    sequence+=(util_parsesubconf)

    # get some information and the ID to use for the following tests
    sequence+=(configqemu_maxvmid)
    sequence+=(client_getnextid)
    sequence+=(client_checkvmref)

    # create a VM and clone it
    sequence+=(configqemu_newconfigqemufromjson)
    sequence+=(client_getnodelist)
    sequence+=(configqemu_createvm)
    sequence+=(configqemu_newconfigqemufromapi)
    sequence+=(client_getnextid)
    sequence+=(configqemu_clonevm)

    # get information about the VMs created
    sequence+=(client_getvmlist)
    sequence+=(client_getvmrefbyname)
    sequence+=(client_checkvmref)
    sequence+=(client_getvminfo)
    sequence+=(client_getvmstate)

    # these two are low level and don't really affect VMs, test their I/O
    sequence+=(configqemu_createqemunetworksparams)
    sequence+=(configqemu_createqemudisksparams)

    # disk operations
    sequence+=(client_createvmdisk)
    sequence+=(client_resizeqemudisk)
    # deletvmdisks will always fail, but testing anyway
    sequence+=(client_deletevmdisks)

    # start the first VM and conduct some tests that need a running VM
    sequence+=(client_startvm)
    sequence+=(configqemu_sshforwardusernet)
    sequence+=(configqemu_sendkeysstring)
    sequence+=(configqemu_removesshforwardusernet)
    sequence+=(client_monitorcmd)

    # continue the status tests and finish with a statuschangevm cycle
    sequence+=(client_startvm)
    sequence+=(client_resetvm)
    sequence+=(client_suspendvm)
    sequence+=(client_resetvm)
    sequence+=(client_suspendvm)
    sequence+=(client_resumevm)
    sequence+=(client_resumevm)
    # this will of course fail if the guest OS doesn't support ACPI
    sequence+=(client_shutdownvm)
    sequence+=(client_stopvm)
    sequence+=(client_stopvm)
    sequence+=(client_statuschangevm)

    # this test requires manual creation of the snapshot
    # because snapshot functionality is not implemented
    sequence+=(client_rollbackqemuvm)

    # session tests
    sequence+=(session_login)
    sequence+=(session_paramstobody)
    sequence+=(session_responsejson)
    sequence+=(session_request)
    sequence+=(session_requestjson)
    sequence+=(session_get)
    sequence+=(session_getjson)
    sequence+=(session_post)
    sequence+=(session_put)
    sequence+=(session_delete)

    # finally delete the VMs and get the task ID
    sequence+=(client_deletevm)
    sequence+=(client_deletevm)

    # get task exit status for the last deletion
    sequence+=(client_gettaskexitstatus)

    for action in "${sequence[@]}"; do
        results[$action]='not yet tested'
        echo $action is ${results[$action]}
      done | sort
}

run_test_sequence() {
    local action setup
    local -A setups=()

    echo -e "\nRunning the tests\n"

    # list all the setup functions
    while read setup; do
        setups[$setup]=1
    done< <(declare -f | sed -rn "s/^(${setup_prefix}.*) \(\)/\1/p")

    # for each entry in the sequence that has a setup, call the setup function
    # if there isn't a setup for the entry, call the default setup

    # setup functions receive as first argument the number of times that
    # function has been called, so they can behave differently depending on
    # the point of execution
    for action in "${sequence[@]}"; do
        setup=${setup_prefix}${action}

        if (( ${setups[$setup]} )); then
            echo -e "\nFound setup for action: \"$action\""
            $setup ${setups[$setup]} $@
            (( setups[$setup]++ ))
        else
            echo -e "\nCalling simple forward for action: \"$action\""
            ${setup_prefix}simple $action
        fi
    done
}

main() {
    echo "Calling the tests with the following environment:"
    echo "PM_API_URL=$PM_API_URL"
    echo "PM_USER=$PM_USER"

    if (( ! $# )); then
        echo -e "\n======= Sequence mode =======\n"

        echo "To avoid entering the password at each test you can enter it at this point"
        echo "If you press enter here, the Go code will ask for the password each time it runs"
        read -sp "Enter the password for $PM_USER: " PM_PASS

        prepare_test_sequence && run_test_sequence
    else
        echo -e "\n======= Forward mode =======\n"
        "$test_binary" $@
    fi
}

# the setup functions are in this file
. "$scriptdir/testsetups"

main $@
