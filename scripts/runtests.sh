#!/bin/bash

# REQUIRES BASH 4.3+

# DESCRIPTION AND USE

# This script can be used to call the proxmox-api-go binary with the PM_API_URL,
# PM_USER and PM_PASS variables set in the environment, to run a specific test
# while forwarding the command line to the binary.
#
# To use the script in this way, run it like this:
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
# The action arguments depend on the specific action to test.

# Alternatively, this script can attempt to run many test actions by following
# a sequence that combines some tests to work together, like for example
# creating a VM, querying information about and changing the VM, then deleting
# it.
#
# To use the script in this way, run it without arguments:
#
# runtests.sh


# SHELL DIRECTIVES
# this parser directive must be always at the top
# enable extended globs for pattern matching
shopt -s extglob


# GLOBAL DATA
declare scriptdir="$(dirname "$0")"

# test setups and their results
declare -a sequence
declare -A results

# to save the last test output (see for example testsetup_capture)
declare testoutput

# columns for error codes and messages
declare -A exitcodes
declare -a exitmsgs

# columns for each VM/CT handled by the tests, all indexed by id
# value of vms entries are vm type ("vm" or "ct")
declare -a vms vmnames vmconfigs
declare selectedid selectedid_vm selectedid_ct

# node finding and selection
declare -a nodes
declare selectednode selectednode_vm selectednode_ct

# storage finding and selection
declare -a storages
declare selectedstorage selectedstorage_vm selectedstorage_ct

# for session POST/PUT/DELETE tests
declare testpoolname='testpool'

# for gettaskexitstatus
declare UPID

# error codes and messages
declare -A exitcodes
declare -a exitmsgs


# CONFIGURATION - modify these values as needed
test_binary="$scriptdir/../proxmox-api-go"
test_default_flags='-debug -insecure'
setup_prefix='testsetup_'
export PM_API_URL='https://10.40.0.147:8006/api2/json' PM_USER='root@pam' PM_PASS


# CODE

# Test sequence functions

# Add rows with names of test actions to the global sequence[] array
# There may be dedicated "setup handler" functions for the target actions, which
# might add extra logic to the test
# See the included testsetups file and the runSequence function below.
# Entries called "end" cause the sequence to be ended at that point
prepareSequence() {
    debugMessage "Preparing the test sequence:"

    sequence=()
    results=()

    local target
    while read entry; do
        [[ -n "$entry" ]] && {
            target="$(xargs <<< $entry)"
            sequence+=("$target")
            setActionResult "$target" NOT_TESTED
        }
    done<<EOF

    node_getnodelist
    node_check
    node_findnode
    node_getinfo

    storage_getstoragelist
    storage_check
    storage_findstorage
    storage_getinfo

    vm_getnextvmid
    configlxc_newconfiglxcfromjson
    configlxc_creatempparams
    configlxc_createnetparams
    configlxc_createvm
    configlxc_newconfiglxcfromapi

    vm_getnextvmid
    configqemu_newconfigqemufromjson
    configqemu_createvm
    configqemu_createdisksparams
    configqemu_createnetparams
    configqemu_newconfigqemufromapi

    vm_getmaxvmid
    vm_getvmlist
    vm_check
    vm_findvm
    vm_getinfo

    node_createvolume
    node_deletevolume
    node_getstorageandvolumename
    vm_movedisk
    vm_resizedisk
    vmdevice_parseconf
    vmdevice_parsesubconf

    vm_getstatus
    vm_start
    vm_reset
    vm_suspend
    vm_resume
    vm_shutdown
    vm_waitforshutdown
    vm_stop

end
    vm_monitorcmd
    vm_sendkeysstring
    vm_sshforwardusernet
    vm_removesshforwardusernet
    vm_getagentnetworkinterfaces
    vm_getspiceproxy

    vm_getconfig
    vm_createsnapshot
    vm_getsnapshotlist
    configlxc_updateconfig
    configqemu_updateconfig
    vm_rollback
    vm_deletesnapshot

    vm_clone
    vm_migrate
    vm_setstatus
    vm_createbackup
    vm_createtemplate
    vm_delete
    client_gettaskexitstatus

    session_login
    session_paramstobody
    session_responsejson
    session_request
    session_requestjson
    session_get
    session_getjson
    session_post
    session_put
    session_delete
end
EOF
}

runSequence() {
    debugMessage "Running the tests"

    local target setup
    local -A setups=()

    # list the test setup handler functions and the times they have been called
    while read setup; do
        setups[$setup]=0
    done< <(declare -f | sed -rn "s/^(${setup_prefix}.*) \(\)/\1/p")

    for target in "${sequence[@]}"; do
        case "$target" in
        end)
            debugMessage "End test sequence"
            break
            ;;
          *)
            echo -e "\nLooking for next action"
            setup=${setup_prefix}${target}

            # if there's a setup handler function for the target, call it
            # otherwise call a default handler
            if [[ -v setups[$setup] ]]; then
                echo -e "Found setup for action: \"$target\""
                $setup $(( ++setups[$setup] )) $@
            else
                echo -e "Calling default handler for action: \"$target\""
                ${setup_prefix}simple $target $@
            fi
        esac
    done
}

printSequence() {
    debugMessage "This is the test sequence:"

    # filter out
    local filter='^\(end\|somethingelse\)'

    {
        echo "Action:Result"
        for action in "${sequence[@]}"; do
            local result="${results[$action]}"
            local output="${action}:$(exitCodeName $result)"
            local msg="${exitmsgs[$result]}"
            [[ -n "$msg" ]] && output="$output ($msg)"
            echo $output
        done | grep -v "$filter" | sort
    } | column -ts:
}

# all non-stub handlers will probably call this at some point
runAction() {
    echo 'Running the test action and capturing output'

    shopt -s lastpipe
    local line
    while read -t 1 line; do
        echo "$line"
    done | "$test_binary" $test_default_flags $@ 2>&1 | readarray -t testoutput< <(cat)
    local target_exit_status=${PIPESTATUS[1]}

    for line in "${testoutput[@]}"; do
        echo "$line"
    done

    return $target_exit_status
}


# Exit codes and messages

# get code name by id
exitCodeName() {
    for code in "${!exitcodes[@]}"; do
        (( ${exitcodes[$code]} == $1 )) && {
            echo $code
        }
    done
}

# add row to exitcodes and exitmsgs
addExitCode() {
    local code=$1 name=$2 ; shift 2 ; local msg="$@"
    exitcodes[$name]=$code
    exitmsgs[$code]="$msg"
}

# rows are added from here at the start of the script
prepareExitCodes() {
    addExitCode 0 PASSED
    addExitCode 1 FAILED
    addExitCode 2000 NOT_TESTED
    addExitCode 2001 MANUALLY_TESTED "the test was conducted by manual intervention"
    addExitCode 2002 STUB "setup handler is a stub"
}

# adds row in the global results table with some result / exit code
#
# exit code:
# $result if it is numeric
# exitcodes[$result] if it is symbolic
setActionResult() {
    local action=$1 result=$2

    # result might be passed as numeric or symbolic name
    re='^[0-9]+$'
    if [[ $result =~ $re ]]; then
        results[$action]=$result
    else
        results[$action]=${exitcodes[$result]}
    fi

    return ${results[$action]}
}


# JSON template setup helpers

# outputs a valid unicast, locally administered MAC address
# parameters:
# $1 - byte delimiter, default ':'
randomMacAddress() {
    local delimiter="${1:-:}"
    local hexchars="0123456789ABCDEF"
    local mac=''
    for i in {1..12} ; do
        mac=$mac${hexchars:$(( $RANDOM % 16 )):1}
    done
    printf '%012X\n' "$(( 0x$mac & 0xFEFFFFFFFFFF | 0x020000000000 ))" \
    | sed -re 's/(..)/\1'$delimiter'/g' -e 's/(.*).$/\1/'
}

# outputs JSON configuration for Qemu VM disks
# parameters:
# $1 - disk name
# $2 - disk size in GB, default 2GB
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

# outputs JSON configuration for CT creation
# parameters:
# $1 - CT name, default 'testct'
ctJson() {
    local ctname="${1:-testct}"
    cat<<EOF
{
    "ostemplate": "local:vztmpl/ubuntu-16.04-standard_16.04-1_amd64.tar.gz",
    "arch": "amd64",
    "cores": 4,
    "hostname": "$ctname",
    "memory": 512,
    "nameserver": "8.8.8.8 1.1.1.1",
    "net": {
        "0": {
        "name": "eth0",
        "bridge": "vmbr0",
        "type": "veth"
        },
        "1": {
        "name": "eth1",
        "bridge": "vmbr0",
        "tag": "25",
        "type":"veth",
        "hwaddr": "$(randomMacAddress)"
        }
    },
    "ostype": "ubuntu",
    "rootfs": {
        "storage": "local-lvm",
        "size": "8G",
        "acl": true
    },
    "searchdomain": "test.com",
    "swap": 512
}
EOF
}

# outputs JSON configuration for Qemu VM creation
# parameters:
# $1 - VM name, default 'testvm'
vmJson() {
    local vmname="${1:-testvm}"
    cat<<EOF
{
    "name": "$vmname",
    "onboot": false,
    "memory": 2048,
    "ostype": "l26",
    "cores": 1,
    "sockets": 1,
    "agent": "enabled=0",
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
            "macaddr": "$(randomMacAddress)" 
        }
    }
}
EOF
}


# Other helpers

# prompts for node selection
# parameters:
# $1 - user message
# $2 - default value for empty selection
# $3 - 1 to forbid entering values not found in the global nodes[] array
#
# exit code:
# 1 = received empty input, global selectednode is "$2"
# 0 = received non-empty input
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

# prompts for storage selection
# parameters:
# $1 - user message
# $2 - default value for empty selection
# $3 - 1 to forbid entering values not found in the global storages[] array
#
# exit code:
# 1 = received empty input, global selectedstorage is "$2"
# 0 = received non-empty input
promptStorage() {
    local message="$1" default="$2" strict="$3"
    while read -p "$message" selectedstorage; do
        case "${selectedstorage}" in
            @($(sed 's/ /|/g' <<< ${storages[@]}))) ;;
            '') selectedstorage="$default"
                return 1 ;;
            *)  if (( $strict )); then
                    echo "\"$selectedstorage\" is not a valid option, please enter one of the following: ${storages[@]}"
                    continue
                fi ;;
        esac
        break
    done

    return 0
}

debugMessage() {
    local msg="$1"
    cat<<EOF

======= $msg =======
EOF
}

startHeader() {
    cat<<EOF
$(debugMessage "$1")

Calling the tests with this environment:

PM_API_URL | $PM_API_URL
PM_USER    | $PM_USER
PM_PASS    | $PM_PASS

EOF
}


# Entry code

main() {
    if (( ! $# )); then
        startHeader "Sequence mode"

        if [[ -z "$PM_PASS" ]]; then
            cat<<EOF
To avoid entering the password at each test you can enter it at this point
If you press enter here, the Go code will ask for the password each time it runs"
EOF
            read -sp "Enter the password for $PM_USER: " PM_PASS
        fi

        prepareExitCodes
        prepareSequence
        runSequence
        printSequence
    else
        startHeader "Forward mode"
        "$test_binary" $@
    fi
}

# include the setup functions in these files
. "$scriptdir/testsetups"

main $@
