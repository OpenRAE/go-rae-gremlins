#!/usr/bin/env bash
#
# Copyright 2022 The Gremlins Authors
#
#    Licensed under the Apache License, Version 2.0 (the "License");
#    you may not use this file except in compliance with the License.
#    You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
#    Unless required by applicable law or agreed to in writing, software
#    distributed under the License is distributed on an "AS IS" BASIS,
#    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#    See the License for the specific language governing permissions and
#    limitations under the License.
#
# run-bounded.sh runs a command inside a Linux cgroup-v2 memory budget.
# It uses a per-user systemd transient service so the kernel ends the whole
# process tree when the budget is reached. The supervisor and the command run as
# the unprivileged invoking user; the script never raises its own privilege and
# never runs the command as root. See docs/adr/0006-bounded-memory-for-mutation-runs.md.
# It needs systemd 243 or later, a unified cgroup-v2 hierarchy, and a per-user
# manager with the memory controller delegated to the user slice.
#
# Usage:
#   scripts/run-bounded.sh --budget SIZE [--slice NAME] [--output-file PATH]
#                          -- COMMAND [ARG...]
#
# On an out-of-memory kill the script prints a clear message, deletes the named
# output file, and exits with code 137. On any other result it passes the command
# exit code through unchanged. A signal to this script stops the unit first.

set -euo pipefail

# OOM_EXIT is the explicit resource-exhaustion exit code of this supervisor.
# It is separate from the Gremlins exit-code contract. The script reports it only
# from the trusted systemd Result, never by inferring an OOM from a signal.
readonly OOM_EXIT=137

usage() {
	cat >&2 <<'EOF'
usage: run-bounded.sh --budget SIZE [--slice NAME] [--output-file PATH]
                      -- COMMAND [ARG...]
EOF
	exit 2
}

# fail prints a message to stderr and exits non-zero without running the command.
fail() {
	printf 'run-bounded: %s\n' "$1" >&2
	exit 1
}

budget=""
slice=""
output_file=""
cmd=()

while [ $# -gt 0 ]; do
	case "$1" in
	--budget)
		[ $# -ge 2 ] || usage
		budget="$2"
		shift 2
		;;
	--slice)
		[ $# -ge 2 ] || usage
		slice="$2"
		shift 2
		;;
	--output-file)
		[ $# -ge 2 ] || usage
		output_file="$2"
		shift 2
		;;
	--)
		shift
		cmd=("$@")
		break
		;;
	*)
		usage
		;;
	esac
done

[ -n "$budget" ] || usage
[ ${#cmd[@]} -ge 1 ] || usage

# This runner must never execute the workload as root. The per-user manager does
# not drop privilege, so reject an effective root caller before anything else.
# Use Bash's readonly EUID, not an external program, so PATH cannot influence it.
[ "$EUID" -ne 0 ] || fail "run-bounded must not run as root; run it as an unprivileged user"

# valid_size accepts a positive systemd size, for example 512M, 8G, or a byte
# count. It rejects zero, infinity, and malformed values before any code runs.
valid_size() {
	case "$1" in
	0 | 0[KMGT] | infinity | max | "")
		return 1
		;;
	esac
	[[ "$1" =~ ^[1-9][0-9]*[KMGT]?$ ]]
}

valid_size "$budget" || fail "invalid --budget '$budget'; use a positive size such as 512M or 8G"

# Reject a budget that is not smaller than the host total memory. A budget at or
# above total RAM cannot end the run before the host is exhausted. The comparison
# runs in the budget's own unit, and an over-long number is rejected first, so the
# fixed-width shell arithmetic cannot overflow.
budget_number="${budget%[KMGT]}"
host_total_bytes=$(($(awk '/^MemTotal:/ {print $2}' /proc/meminfo) * 1024))
case "$budget" in
*K) host_limit=$((host_total_bytes / 1024)) ;;
*M) host_limit=$((host_total_bytes / (1024 * 1024))) ;;
*G) host_limit=$((host_total_bytes / (1024 * 1024 * 1024))) ;;
*T) host_limit=$((host_total_bytes / (1024 * 1024 * 1024 * 1024))) ;;
*) host_limit=$host_total_bytes ;;
esac
if [ "${#budget_number}" -gt 18 ] || [ "$budget_number" -ge "$host_limit" ]; then
	fail "--budget '$budget' is not smaller than host memory; choose a budget below total RAM"
fi

if [ -n "$slice" ] && ! [[ "$slice" =~ ^[a-zA-Z0-9][a-zA-Z0-9-]*\.slice$ ]]; then
	fail "invalid --slice '$slice'; use a systemd slice name such as gremlins.slice"
fi

command -v systemd-run >/dev/null 2>&1 || fail "systemd-run not found; a Linux systemd manager is required"
command -v systemctl >/dev/null 2>&1 || fail "systemctl not found; a Linux systemd manager is required"

# The unified cgroup-v2 hierarchy is required for MemoryMax and MemorySwapMax.
[ -f /sys/fs/cgroup/cgroup.controllers ] || fail "the unified cgroup-v2 hierarchy is required"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$EUID}"

# The runner uses only the per-user manager, so the command runs as the
# unprivileged invoking user. The script never raises privilege, never uses sudo,
# and never runs the command as root.
user_available() {
	# show-environment succeeds when the per-user manager is reachable, including
	# the degraded state, so one failed unit does not hide it.
	systemctl --user show-environment >/dev/null 2>&1
}
user_available || fail "the per-user systemd manager is not available; enable it with 'loginctl enable-linger'"
scope_flag=(--user)

slice_arg=()
[ -n "$slice" ] && slice_arg=("--slice=$slice")

# verify_controller proves the memory controller is effective before the workload
# runs. It starts a short unit with the budget and reads the live memory.max and
# memory.swap.max from inside the unit. A missing or infinite memory limit, or a
# non-zero swap limit, fails closed.
verify_controller() {
	local probe_unit="run-bounded-probe-$$-${RANDOM}.service"
	local observed="" max="" swap=""
	# The $(...) inside the single-quoted program runs inside the unit, by design.
	# shellcheck disable=SC2016
	observed="$(systemd-run "${scope_flag[@]}" "${slice_arg[@]}" --quiet --wait --pipe --collect \
		-p MemoryMax="$budget" -p MemorySwapMax=0 \
		--unit="$probe_unit" -- \
		sh -c 'cg="/sys/fs/cgroup$(cut -d: -f3 </proc/self/cgroup)"; printf "%s;%s" "$(cat "$cg/memory.max" 2>/dev/null)" "$(cat "$cg/memory.swap.max" 2>/dev/null)"' 2>/dev/null || true)"
	systemctl "${scope_flag[@]}" reset-failed "$probe_unit" >/dev/null 2>&1 || true
	max="${observed%%;*}"
	swap="${observed##*;}"
	# The memory limit must be a finite byte count and swap must be denied.
	[[ "$max" =~ ^[0-9]+$ ]] || return 1
	[ "$swap" = "0" ]
}

verify_controller || fail "the memory controller did not apply a finite limit and a zero swap limit; enforcement is not available"

unit="gremlins-bounded-$$-${RANDOM}.service"
# The supervisor owns an isolated temporary root. A group-wide OOM kill bypasses
# the workload's own cleanup defers, so the supervisor removes this root itself.
work_root="$(mktemp -d "${TMPDIR:-/tmp}/run-bounded-XXXXXX")"
sr_pid=""

# These three functions run from traps, so shellcheck cannot see the call site.
# shellcheck disable=SC2329
remove_work_root() {
	rm -rf -- "$work_root"
}

# shellcheck disable=SC2329
stop_unit() {
	# A signal to this script does not stop the manager-owned unit, so stop it
	# here. systemctl stop is synchronous and ends descendants via KillMode.
	systemctl "${scope_flag[@]}" stop "$unit" >/dev/null 2>&1 || true
	systemctl "${scope_flag[@]}" reset-failed "$unit" >/dev/null 2>&1 || true
}

# shellcheck disable=SC2329
on_signal() {
	[ -n "$sr_pid" ] && kill "$sr_pid" >/dev/null 2>&1 || true
	stop_unit
	remove_work_root
	exit 143
}

trap remove_work_root EXIT
trap on_signal INT TERM

# Build the workload with a clean environment. A per-user service inherits the
# manager environment, so env -i clears it and sets only the caller's allowlisted
# values. This keeps the workload behaviorally transparent to the operator's
# Gremlins and non-secret Go settings while isolating it from the manager
# environment. A value that carries URL credentials is skipped, and the manager
# bus address is not forwarded.
env_bin="$(command -v env)" || fail "env not found; it is required to build a clean workload environment"
forward_allowlist=(
	PATH HOME USER LOGNAME LANG TERM LC_ALL LC_CTYPE XDG_CONFIG_HOME
	GOROOT GOPATH GOCACHE GOMODCACHE GOTOOLCHAIN GOFLAGS
	GO111MODULE GOWORK GODEBUG GOEXPERIMENT GOOS GOARCH GOARM GOAMD64
	GOPROXY GOSUMDB GOPRIVATE GONOSUMDB GONOSUMCHECK GOINSECURE GOENV
	CGO_ENABLED CC CXX CGO_CFLAGS CGO_CPPFLAGS CGO_CXXFLAGS CGO_LDFLAGS
)
workload=("$env_bin" -i)
# Forward the explicit allowlist and every GREMLINS_* configuration variable, so
# a bounded run honors the same settings as a direct Gremlins command.
for name in "${forward_allowlist[@]}" ${!GREMLINS_@}; do
	[ -n "${!name+x}" ] || continue
	# Reject any URL userinfo, with or without a password, so a token or a
	# credential never becomes a process argument on the systemd-run command line.
	if printf '%s' "${!name}" | grep -qE '://[^/@[:space:]]+@'; then
		printf 'run-bounded: skipped %s because its value looks credential-bearing\n' "$name" >&2
		continue
	fi
	workload+=("$name=${!name}")
done
# Point the workload's temporary files at the supervisor-owned root. Set only
# TMPDIR: the go command derives its own temporary directory from TMPDIR, and
# Gremlins sets a per-mutant GOTMPDIR itself, so the supervisor must not clobber
# GOTMPDIR or a Gremlins self-test that asserts it would fail.
workload+=("TMPDIR=$work_root")
workload+=("${cmd[@]}")

# Escape every dollar sign so systemd treats the whole command line as data and
# does not expand an environment reference before env runs.
escaped_cmd=()
for arg in "${workload[@]}"; do
	escaped_cmd+=("${arg//\$/\$\$}")
done

# The real unit runs in the background so a signal can interrupt the wait and
# stop the unit. It runs without --collect so a failed unit keeps its Result for
# the trusted OOM read below; reset-failed then removes it.
run_rc=0
systemd-run "${scope_flag[@]}" "${slice_arg[@]}" --wait --pipe \
	-p MemoryMax="$budget" \
	-p MemorySwapMax=0 \
	-p MemoryHigh=infinity \
	-p OOMPolicy=kill \
	-p KillMode=control-group \
	-p Restart=no \
	--working-directory="$PWD" \
	--unit="$unit" \
	-- "${escaped_cmd[@]}" &
sr_pid=$!
wait "$sr_pid" && run_rc=0 || run_rc=$?
sr_pid=""
trap - INT TERM

# Read the trusted systemd result. An OOM kill is the only resource-exhaustion
# signal; the script never infers it from the exit code or a signal.
result="$(systemctl "${scope_flag[@]}" show -p Result --value "$unit" 2>/dev/null || true)"
systemctl "${scope_flag[@]}" reset-failed "$unit" >/dev/null 2>&1 || true

if [ "$result" = "oom-kill" ]; then
	printf 'run-bounded: out of memory; the %s budget was reached and the run was ended\n' "$budget" >&2
	if [ -n "$output_file" ] && [ -e "$output_file" ]; then
		rm -f -- "$output_file"
		printf 'run-bounded: deleted the partial output file %s\n' "$output_file" >&2
	fi
	exit "$OOM_EXIT"
fi

exit "$run_rc"
