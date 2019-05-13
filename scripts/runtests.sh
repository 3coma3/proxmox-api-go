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

declare testoutput

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
prepareSequence() {
    debugMessage "Preparing the test sequence:"

    sequence+=(node_getnodelist)
    sequence+=(node_findnode)
    sequence+=(node_check)
    sequence+=(node_getinfo)

    sequence+=(configlxc_newconfiglxcfromapi)
    sequence+=(end)

    sequence+=(configlxc_createvm)
    sequence+=(configlxc_newconfiglxcfromjson)
    sequence+=(configlxc_creatempparams)
    sequence+=(configlxc_createnetparams)

    sequence+=(configqemu_newconfigqemufromjson)
    sequence+=(configqemu_createvm)
    sequence+=(configqemu_newconfigqemufromapi)
    sequence+=(configqemu_createdisksparams)
    sequence+=(configqemu_createnetparams)

    sequence+=(vm_getvmlist)
    sequence+=(vm_getmaxvmid)
    sequence+=(vm_getnextvmid)
    sequence+=(vm_check)
    sequence+=(vm_findvm)
    sequence+=(vm_getinfo)

    sequence+=(node_createvolume)
    sequence+=(node_deletevolume)
    sequence+=(node_getstorageandvolumename)
    sequence+=(vm_movedisk)
    sequence+=(vm_resizedisk)
    sequence+=(vmdevice_parseconf)
    sequence+=(vmdevice_parsesubconf)

    sequence+=(vm_getstatus)
    sequence+=(vm_start)
    sequence+=(vm_reset)
    sequence+=(vm_suspend)
    sequence+=(vm_resume)
    sequence+=(vm_shutdown)
    sequence+=(vm_waitforshutdown)
    sequence+=(vm_stop)
    sequence+=(vm_setstatus)

    sequence+=(vm_monitorcmd)
    sequence+=(vm_sendkeysstring)
    sequence+=(vm_sshforwardusernet)
    sequence+=(vm_removesshforwardusernet)
    sequence+=(vm_getagentnetworkinterfaces)
    sequence+=(vm_getspiceproxy)

    sequence+=(vm_getconfig)
    sequence+=(vm_createsnapshot)
    sequence+=(vm_getsnapshotlist)
    sequence+=(configlxc_updateconfig)
    sequence+=(configqemu_updateconfig)
    sequence+=(vm_rollback)
    sequence+=(vm_deletesnapshot)

    sequence+=(vm_clone)
    sequence+=(vm_migrate)
    sequence+=(vm_createbackup)
    sequence+=(vm_createtemplate)
    sequence+=(vm_delete)
    sequence+=(client_gettaskexitstatus)

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

    local action
    for action in "${sequence[@]}"; do
        results[$action]='not yet tested'
    done
}

runSequence() {
    local action setup
    local -A setups=()

    debugMessage "Running the tests"

    # save all the test setup functions
    while read setup; do
        setups[$setup]=1
    done< <(declare -f | sed -rn "s/^(${setup_prefix}.*) \(\)/\1/p")

    # for each entry in the sequence that has a setup, call the setup function
    # otherwise pass the entry to the default setup as its target

    # setup functions receive the number of times that they have been called so
    # they can behave differently depending on the point of execution in the
    # sequence, this is their first argument
    for action in "${sequence[@]}"; do
        [[ $action == end ]] && {
            debugMessage "End sequence"
            return
        }

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

printSequence() {
    debugMessage "This is the test sequence:"

    # don't print these
    local filter='^\(end\|somethingelse\)'

    for action in "${sequence[@]}"; do
       echo "$action -> ${results[$action]}"
    done | grep -v "$filter" | sort
}

promptNode() {
    local message="$1" default="$2" strict="$3"
    while read -p "$message" selectednode; do
        case "${selectednode}" in
            @($(sed 's/ /|/g' <<< ${nodes[@]}))) ;;
            '') selectednode="$default"
                return 1 ;;
            *)  if (( $strict )); then
                    echo "\"$selectednode\" is not a valid option, please enter one of the following: ${nodes[@]}"
                    continue
                fi ;;
        esac
        break
    done

    return 0
}

setActionResult() {
  local target=$1 result=$2
    case "$result" in
        0) results[$target]="PASSED" ;;
        1) results[$target]="FAILED" ;;
        *) results[$target]="OTHER - test action has returned exit status = $result" ;;
    esac
}

debugMessage() {
    local msg="$1"
    cat<<EOF


======= $msg =======

EOF
}

startHeader() {
    cat<<EOF
$(debugMessage "$1 mode")

Calling the tests with this environment:

PM_API_URL | $PM_API_URL
PM_USER    | $PM_USER
PM_PASS    | $PM_PASS

EOF
}

main() {
    if (( ! $# )); then
        startHeader Sequence

        if [[ -z "$PM_PASS" ]]; then
            echo "To avoid entering the password at each test you can enter it at this point"
            echo "If you press enter here, the Go code will ask for the password each time it runs"
            read -sp "Enter the password for $PM_USER: " PM_PASS
        fi

        prepareSequence; printSequence; runSequence; printSequence
    else
        startHeader Forward
        "$test_binary" $@
    fi
}

# the setup functions are in this file
. "$scriptdir/testsetups"

main $@
