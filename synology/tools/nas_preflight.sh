#!/bin/sh
# Read-only DS216 gate. No credentials, package writes, login or service changes.
# This is an operator preflight, NOT attestation of a running WorkBridge service.
set -eu

MODEL_FILE="${WORKBRIDGE_PREFLIGHT_MODEL_FILE:-/proc/sys/kernel/syno_hw_version}"
MEMINFO_FILE="${WORKBRIDGE_PREFLIGHT_MEMINFO_FILE:-/proc/meminfo}"
DSM_VERSION_FILE="${WORKBRIDGE_PREFLIGHT_VERSION_FILE:-/etc.defaults/VERSION}"
PACKAGE_DIR="${WORKBRIDGE_PREFLIGHT_PACKAGE_DIR:-/var/packages/WorkBridgeRelay}"
VOLUME="${WORKBRIDGE_PREFLIGHT_VOLUME:-/volume1}"
MIN_FREE_KIB=524288
MIN_AVAILABLE_KIB=196608

error=0
note() { printf '%s=%s\n' "$1" "$2"; }
reject() { note "$1" "$2"; error=1; }

if [ ! -r "$MODEL_FILE" ]; then
    reject hardware_model unknown
else
    model=$(cat "$MODEL_FILE" 2>/dev/null || true)
    note hardware_model "$model"
    [ "$model" = "DS216" ] || reject hardware_gate not_original_ds216
fi

arch=$(uname -m)
note kernel_arch "$arch"
case "$arch" in
    armv7l|armv7|armv7*) ;;
    *) reject arch_gate not_armv7 ;;
esac

if [ ! -r "$DSM_VERSION_FILE" ]; then
    reject dsm_version unknown
else
    # Do not source DSM's VERSION file or execute it as shell code.
    major=$(sed -n 's/^majorversion="\([0-9][0-9]*\)".*/\1/p' "$DSM_VERSION_FILE" | head -n 1)
    minor=$(sed -n 's/^minorversion="\([0-9][0-9]*\)".*/\1/p' "$DSM_VERSION_FILE" | head -n 1)
    build=$(sed -n 's/^buildnumber="\([0-9][0-9]*\)".*/\1/p' "$DSM_VERSION_FILE" | head -n 1)
    note dsm_version "${major:-unknown}.${minor:-unknown}-${build:-unknown}"
    if [ "$major" != "7" ] || [ -z "$minor" ] || [ -z "$build" ]; then
        reject dsm_gate not_verified_7_2_72806
    elif [ "$minor" -lt 2 ] || [ "$build" -lt 72806 ]; then
        reject dsm_gate below_package_minimum
    fi
fi

total=
available=
if [ -r "$MEMINFO_FILE" ]; then
    total=$(awk '$1=="MemTotal:" && $2 ~ /^[0-9]+$/ {print $2;exit}' "$MEMINFO_FILE")
    available=$(awk '$1=="MemAvailable:" && $2 ~ /^[0-9]+$/ {print $2;exit}' "$MEMINFO_FILE")
fi
note memory_total_kib "${total:-unknown}"
note memory_available_kib "${available:-unknown}"
if [ -z "$total" ]; then
    reject memory_gate memtotal_unavailable
elif [ "$total" -lt 380000 ] || [ "$total" -gt 620000 ]; then
    reject memory_gate unexpected_for_512mb_ds216
fi
if [ -z "$available" ]; then
    reject memory_headroom_gate unknown_recheck_in_dsm_resource_monitor
elif [ "$available" -lt "$MIN_AVAILABLE_KIB" ]; then
    reject memory_headroom_gate low_available_memory
fi

if [ -e "$PACKAGE_DIR" ] || [ -L "$PACKAGE_DIR" ]; then
    reject package_state already_present_no_implicit_upgrade
else
    note package_state absent
fi

if [ ! -d "$VOLUME" ]; then
    reject volume_gate missing_target_volume
else
    free_kib=$(df -Pk "$VOLUME" 2>/dev/null | awk 'NR==2 && $4 ~ /^[0-9]+$/ {print $4}')
    note volume_free_kib "${free_kib:-unknown}"
    if [ -z "$free_kib" ] || [ "$free_kib" -lt "$MIN_FREE_KIB" ]; then
        reject volume_gate insufficient_verified_free_space
    fi
fi

if [ "$error" -ne 0 ]; then
    note preflight NOT_READY
    exit 2
fi
note preflight PREPARED_ONLY
note installation_state NOT_VERIFIED
exit 0
