#!/bin/bash

# REQUIRES BASH 4.3+

# DESCRIPTION AND USE ----------------------------------------------------------

# This script can be used to call the proxmox-api-go binary with the PM_API_URL,
# PM_USER and PM_PASS variables set in the environment, to run a specific test
# while forwarding the command line to the binary.

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
# a suite that combines some tests to work together, like for example creating a
# virtual machine, querying information about and changing it, then deleting it.

# To use the script in this way, run it either specifying a valid suite name
# (one of the strings after the underscore in the files named suite_* in the
# scripts subdirectory). If no suite name is specified (empty arguments), the
# suite pointed to by the global $defaultsuite is run.
#
# runtests.sh [suitename]


# SHELL DIRECTIVES -------------------------------------------------------------

# this parser directive must be always at the top
# enable extended globs for pattern matching
shopt -s extglob


# GLOBAL DATA ------------------------------------------------------------------

declare scriptdir="$(dirname "$0")"

# suite of test setups and their results
declare -a suite
declare -A results

# to save the last test output - see runAction() below and the test setups
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

# for session request tests
declare selectedendpoint='/storage' selectedendpointparams='type=lvmthin'

# for session POST/PUT/DELETE tests
declare selectedpool='testpool'

# for gettaskexitstatus tests
declare -a UPIDs


# CONFIGURATION - modify these values as needed --------------------------------

export PM_API_URL='https://10.40.0.147:8006/api2/json' PM_USER='root@pam' PM_PASS

test_default_flags='-debug -insecure'       # see proxmox-api-go documentation
defaultsuite="full"                         # see scripts/suite_*
setup_prefix='testsetup_'                   # see scripts/testsetups* and here
defaultsetup="${setup_prefix}simple"        # see scripts/testsetups
test_binary="$scriptdir/../proxmox-api-go"


# CODE -------------------------------------------------------------------------

# JSON template setup helpers

# outputs a valid unicast, locally administered MAC address
# parameters:
# $1 - byte delimiter, default ':'
randomMacAddress() {
    local delimiter="${1:-:}" hex="0123456789ABCDEF" bytes=6 mac=''

    while (( bytes-- )); do
        mac=$mac${hex:$(( $RANDOM % 16 )):1}${hex:$(( $RANDOM % 16 )):1}
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
ctCreateJson() {
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
        "size": "5G",
        "acl": true
    },
    "searchdomain": "test.com",
    "swap": 512
}
EOF
}

# outputs JSON configuration for CT reconfiguration
# parameters:
# $1 - CT name, default 'testct'
ctUpdateConfigJson() {
    local ctname="${1:-testct}"
    cat<<EOF
{
    "hostname": "$ctname",
    "console": true,
    "cores": 4,
    "cpuunits": 20248,
    "memory": 2048,
    "ostype": "ubuntu",
    "protection": false,
    "swap": 1024,
    "tty": 2,
    "arch": "amd64",
    "nameserver": "1.1.1.1",
    "net": {
        "1": {
        "name": "eth1",
        "bridge": "vmbr0",
        "tag": "26",
        "type":"veth",
        "hwaddr": "$(randomMacAddress)"
        }
    },
    "searchdomain": "test2.com"
}
EOF
}

# outputs JSON for Qemu VM creation
# parameters:
# $1 - VM name, default 'testvm'
vmCreateJson() {
    local vmname="${1:-testvm}"
    cat<<EOF
{
    "name": "$vmname",
    "onboot": false,
    "memory": 2048,
    "ostype": "l26",
    "cores": 1,
    "sockets": 1,
    "agent": "enabled=1",
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

# outputs JSON for Qemu VM reconfiguration
# parameters:
# $1 - VM name, default 'testvm'
vmUpdateConfigJson() {
    local vmname="${1:-testvm}"
    cat<<EOF
{
    "name": "$vmname",
    "onboot": false,
    "memory": 3072,
    "ostype": "l26",
    "cores": 4,
    "sockets": 2,
    "agent": "enabled=0",
    "delete": "ide2",
    "network": {
        "0": {
            "model": "e1000",
            "bridge": "vmbr0",
            "macaddr": "$(randomMacAddress)" 
        }
    }
}
EOF
}


# Other helpers

# prompts for the PM_PASS variable for the Go code if it's unset
promptPmPass() {
    if [[ -z "$PM_PASS" ]]; then
        cat<<EOF
To avoid entering the password at each test you can enter it at this point
If you press enter here, the Go code will ask for the password each time it runs
EOF
        read -sp "Enter the password for $PM_USER: " PM_PASS
    fi
}

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

# prompts for endpoint selection
promptEndpoint() {
    local line 

    debugMessage 'Manual intervention needed'
    cat<<EOF
For this test you can supply a REST endpoint and parameters to issue a GET request
if you press enter here I will use these defaults:

endpoint: $selectedendpoint
parameters: $selectedendpointparams

The parameter list should be a comma delimited list of key=value pairs, with no spaces

I will use the defaults if nothing is entered after 20 seconds
EOF

    read -t 20 -p "What endpoint should I query? ($selectedendpoint) " line
    [[ -n "$line" ]] && {
      selectedendpoint="$line"
      read -t 20 -p "What parameter should I send in the query? ($params) " line
      [[ -n "$line" ]] && selectedendpointparams="$line"
    }

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


# Test suite functions

# Add rows with names of test actions to the global suite[] and results[] arrays
# There may be dedicated "setup handler" functions for the target actions, which
# add extra logic to the test. See the included testsetups files and the rest of
# the suite-related functions.
prepareSuite() {
    debugMessage "Preparing the test suite"

    local selectedsuite="$1"

    suite=()
    results=()

    local target line
    while read line; do
        case "$line" in
            # comments and blank lines
            @(''|#*)) continue ;;
            # end parsing
                 end) break ;;
            # other lines: trim whitespace, set rows
                   *) target="$(xargs <<< $line)"
                      suite+=("$target")
                      setActionResult "$target" NOT_TESTED ;;
        esac
    done< "$scriptdir/suite_${selectedsuite}"
}

printSuite() {
    debugMessage "This is the test suite"

    # filter out
    local filter='^\(comment\|somethingelse\)'

    local action result output step=1

    {
        echo "Step:Action:Result"
        for action in "${suite[@]}"; do
            result="${results[$action]}"
            output="$((step++)):${action}:$(exitCodeName $result)"
            [[ -n "${exitmsgs[$result]}" ]] && output="$output (${exitmsgs[$result]})"
            echo $output
        done | grep -v "$filter" | sort -n
    } | column -ts:
}

runSuite() {
    debugMessage "Running the tests"

    local -A setups=()
    local target setup

    # list the test setup handler functions and the times they have been called
    while read setup; do
        setups[$setup]=0
    done< <(declare -f | sed -rn "s/^(${setup_prefix}.*) \(\)/\1/p")

    for target in "${suite[@]}"; do
        echo -e "\n$FUNCNAME: Looking for next action"

        setup="${setup_prefix}${target}"
        

        if [[ -v setups[$setup] ]]; then
            echo -e "$FUNCNAME: Found setup for action: \"$target\""
            $setup $(( ++setups[$setup] )) $@
        else
            echo -e "$FUNCNAME: Calling default handler for action: \"$target\""
            $defaultsetup $target $@
        fi
    done
}

# all non-stub handlers will probably call this at some point
# FIXME: add comments for this function
runAction() {
    echo -e "\n$FUNCNAME: Running the test action and capturing output"

    shopt -s lastpipe
    local line
    while read -t 1 line; do
        echo "$line"
    done | "$test_binary" $test_default_flags $@ 2>&1 | readarray -t testoutput
    local target_exit_status=${PIPESTATUS[1]}

    echo "$FUNCNAME: This is the test output:"
    for line in "${testoutput[@]}"; do
        echo "$line"
    done

    return $target_exit_status
}

# adds row in the global results table with some result / exit code
# utimately gets passed the target exit status gotten from runAction
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

# exit name/code/msg rows are added at main() using this function
prepareExitCodes() {
    addExitCode 0 PASSED
    addExitCode 1 FAILED
    addExitCode 2000 NOT_TESTED
    addExitCode 2001 MANUALLY_TESTED "the test was conducted by manual intervention"
    addExitCode 2002 STUB "setup handler is a stub"
}


# Entry code

main() {
    local suite=$defaultsuite

    # a first parameter might be allowed to specify a valid suite for suite mode
    [[ -f "$scriptdir/suite_${1}" ]] && {
        suite=$1
        shift
    }

    if (( ! $# )); then
        startHeader "Suite mode"

        promptPmPass
        prepareExitCodes

        prepareSuite $suite
        runSuite
        printSuite
    else
        startHeader "Forward mode"

        "$test_binary" $@
    fi
}

# include the setup functions in these files
. "$scriptdir/testsetups"

main $@
