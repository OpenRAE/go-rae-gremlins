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
# memory-budget-check.sh proves that scripts/run-bounded.sh enforces a memory
# budget. It runs a small controlled allocator inside an enforced outer slice and
# checks the clear run result. It never sizes an experiment to exhaust the host.
# See docs/adr/0006-bounded-memory-for-mutation-runs.md.

set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."
readonly RUN_BOUNDED="scripts/run-bounded.sh"

export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"

# Inner budgets. The allocator target of 256 MiB exceeds OOM_BUDGET and fits
# FIT_BUDGET, so the same allocator proves the limit, not the allocator, causes
# the kill. OUTER is the independent enforced outer bound; it exceeds every inner
# budget with headroom so a containment defect cannot exhaust the host.
readonly OOM_BUDGET="64M"
readonly FIT_BUDGET="512M"
readonly OUTER="1G"
readonly SMALL_MIB=16
readonly BIG_MIB=256

readonly OUTER_SLICE="grembudget-$$.slice"
readonly KEEPER="grembudget-keeper-$$.service"

fail=0
tmp="$(mktemp -d)"

cleanup() {
	systemctl --user stop "$KEEPER" >/dev/null 2>&1 || true
	systemctl --user stop "$OUTER_SLICE" >/dev/null 2>&1 || true
	# Reset only this check's own units, not unrelated failure state.
	systemctl --user reset-failed "$KEEPER" "$OUTER_SLICE" >/dev/null 2>&1 || true
	rm -rf -- "$tmp"
}
trap cleanup EXIT

note() { printf '[memcheck] %s\n' "$1"; }

# check runs a labelled case and compares the exit code with the wanted value.
check() {
	local want="$1" label="$2"
	shift 2
	local rc=0
	timeout 90 "$@" >/dev/null 2>&1 || rc=$?
	if [ "$rc" = "$want" ]; then
		note "PASS: $label (exit $rc)"
	else
		note "FAIL: $label (exit $rc, want $want)"
		fail=1
	fi
}

pass_or_fail() {
	if [ "$1" = ok ]; then
		note "PASS: $2"
	else
		note "FAIL: $2"
		fail=1
	fi
}

require_capability() {
	command -v systemd-run >/dev/null 2>&1 || {
		note "FAIL: systemd-run is not available; memory enforcement cannot be proven"
		exit 1
	}
	command -v python3 >/dev/null 2>&1 || {
		note "FAIL: python3 is not available for the controlled allocator"
		exit 1
	}
	# A functional probe of run-bounded. A missing controller fails closed here.
	if ! "$RUN_BOUNDED" --budget "$OOM_BUDGET" -- /bin/true >/dev/null 2>&1; then
		note "FAIL: run-bounded could not establish a memory budget; the controller is unavailable"
		note "A skipped capability check is not acceptance evidence."
		exit 1
	fi
}

# establish_outer_bound creates and verifies an independent enforced outer limit
# before any allocator runs. The inner experiments run inside this slice.
establish_outer_bound() {
	systemd-run --user --unit="$KEEPER" --slice="$OUTER_SLICE" -- /bin/sleep infinity >/dev/null 2>&1 ||
		{ note "FAIL: could not create the outer slice"; exit 1; }
	systemctl --user set-property "$OUTER_SLICE" MemoryMax="$OUTER" MemorySwapMax=0 >/dev/null 2>&1 ||
		{ note "FAIL: could not set the outer slice budget"; exit 1; }
	local applied applied_swap
	applied="$(systemctl --user show -p MemoryMax --value "$OUTER_SLICE" 2>/dev/null || true)"
	applied_swap="$(systemctl --user show -p MemorySwapMax --value "$OUTER_SLICE" 2>/dev/null || true)"
	case "$applied" in
	"" | infinity | max | 18446744073709551615)
		note "FAIL: the outer slice memory limit is not enforced (MemoryMax=$applied)"
		exit 1
		;;
	esac
	if [ "$applied_swap" != 0 ]; then
		note "FAIL: the outer slice swap limit is not zero (MemorySwapMax=$applied_swap)"
		exit 1
	fi
	note "outer safety bound established and verified: slice MemoryMax=$applied bytes, swap limit 0"
}

write_allocator() {
	cat >"$tmp/alloc.py" <<'PY'
import sys

# alloc.py allocates MiB of memory in 4 MiB chunks and touches every page, so
# virtual reservation alone cannot satisfy the experiment. It caps its own
# allocation at 2048 MiB as a safety bound independent of the cgroup limit.
mib = min(int(sys.argv[1]), 2048)
partial = sys.argv[2] if len(sys.argv) > 2 else None
if partial:
    with open(partial, "w") as handle:
        handle.write("partial report\n")
chunks = []
for _ in range(mib // 4):
    block = bytearray(4 * 1024 * 1024)
    for offset in range(0, len(block), 4096):
        block[offset] = 1
    chunks.append(block)
print("COMPLETED", mib, "MiB")
PY
}

# cancel_case proves a signal to the supervisor stops the manager-owned unit.
cancel_case() {
	"$RUN_BOUNDED" --budget 256M --slice "$OUTER_SLICE" -- /bin/sh -c 'sleep 60' >/dev/null 2>&1 &
	local rbpid=$!
	local glob="gremlins-bounded-${rbpid}-*"
	local started=0
	for _ in $(seq 1 50); do
		if systemctl --user list-units --all --no-legend "$glob" 2>/dev/null | grep -q .; then
			started=1
			break
		fi
		sleep 0.1
	done
	if [ "$started" != 1 ]; then
		kill "$rbpid" >/dev/null 2>&1 || true
		pass_or_fail bad "cancellation setup: the workload unit never started"
		return
	fi
	kill -TERM "$rbpid" >/dev/null 2>&1 || true
	wait "$rbpid" 2>/dev/null || true
	local gone=0
	for _ in $(seq 1 50); do
		if ! systemctl --user list-units --all --no-legend "$glob" 2>/dev/null | grep -q .; then
			gone=1
			break
		fi
		sleep 0.1
	done
	if [ "$gone" = 1 ]; then
		pass_or_fail ok "a signal to the supervisor stops the workload unit"
	else
		pass_or_fail bad "a signal to the supervisor stops the workload unit"
	fi
}

main() {
	note "memory budget acceptance check"
	require_capability
	establish_outer_bound
	write_allocator
	local alloc="python3 -I $tmp/alloc.py"
	local sl=(--slice "$OUTER_SLICE")

	# Enforcement: the allocator exceeds the budget and the unit is OOM-killed.
	# shellcheck disable=SC2086
	check 137 "enforced limit OOM-kills the allocator" \
		"$RUN_BOUNDED" --budget "$OOM_BUDGET" "${sl[@]}" -- $alloc "$BIG_MIB"

	# Child-tree coverage: a descendant in a new process group stays bound.
	check 137 "descendant in a new process group is bound" \
		"$RUN_BOUNDED" --budget "$OOM_BUDGET" "${sl[@]}" -- \
		/bin/sh -c "setsid python3 -I $tmp/alloc.py $BIG_MIB & wait"

	# Child-tree coverage: a grandchild command inherits the budget.
	check 137 "grandchild command inherits the budget" \
		"$RUN_BOUNDED" --budget "$OOM_BUDGET" "${sl[@]}" -- \
		/bin/sh -c "sh -c 'python3 -I $tmp/alloc.py $BIG_MIB'"

	# Control: the same allocator fits under a sufficient budget and completes.
	# shellcheck disable=SC2086
	check 0 "same allocator completes under a sufficient budget" \
		"$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- $alloc "$BIG_MIB"

	# Normal success: a small allocator under a sufficient budget completes.
	# shellcheck disable=SC2086
	check 0 "small allocator completes (normal success)" \
		"$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- $alloc "$SMALL_MIB"

	# Partial-report rejection: the output file is deleted after an OOM.
	local report="$tmp/report.json"
	printf 'stale\n' >"$report"
	# shellcheck disable=SC2086
	check 137 "out-of-memory run deletes the partial output" \
		"$RUN_BOUNDED" --budget "$OOM_BUDGET" "${sl[@]}" --output-file "$report" -- $alloc "$BIG_MIB" "$report"
	if [ -e "$report" ]; then
		pass_or_fail bad "the partial output file is deleted after the OOM"
	else
		pass_or_fail ok "the partial output file is deleted after the OOM"
	fi

	# Work-directory cleanup: the supervisor removes its temporary root after OOM.
	local wr="$tmp/wr"
	mkdir -p "$wr"
	# $TMPDIR is expanded inside the unit shell, by design.
	# shellcheck disable=SC2016
	TMPDIR="$wr" timeout 90 "$RUN_BOUNDED" --budget "$OOM_BUDGET" "${sl[@]}" -- \
		/bin/sh -c 'echo leak > "$TMPDIR/leak.txt"; python3 -I '"$tmp"'/alloc.py '"$BIG_MIB" >/dev/null 2>&1 || true
	if find "$wr" -maxdepth 1 -name 'run-bounded-*' 2>/dev/null | grep -q .; then
		pass_or_fail bad "the supervisor removes its temporary root after the OOM"
	else
		pass_or_fail ok "the supervisor removes its temporary root after the OOM"
	fi

	# Ordinary failure: a non-OOM failure passes its exit code through.
	check 1 "ordinary failure is not reported as an OOM" \
		"$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /bin/sh -c 'exit 1'

	# Application threshold failure: exit 10 passes through unchanged.
	check 10 "threshold exit code passes through" \
		"$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /bin/sh -c 'exit 10'

	# Negative control: a non-OOM child that exits 137 is not treated as an OOM.
	# The supervisor's own OOM code is also 137, so the distinguishing observable is
	# that the output file survives when the systemd Result is not oom-kill.
	local control="$tmp/control.json"
	printf 'kept\n' >"$control"
	check 137 "a non-OOM exit 137 passes through" \
		"$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" --output-file "$control" -- /bin/sh -c 'exit 137'
	if [ -e "$control" ]; then
		pass_or_fail ok "a non-OOM exit 137 does not delete the output (OOM comes from Result)"
	else
		pass_or_fail bad "a non-OOM exit 137 does not delete the output (OOM comes from Result)"
	fi

	# Argument handling: systemd does not expand a dollar sign in a workload argument.
	# The literal $HOME argument must come back unchanged, not the expanded value.
	local dollar_out
	# shellcheck disable=SC2016
	dollar_out="$(timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/printf '%s' '$HOME' 2>/dev/null || true)"
	# shellcheck disable=SC2016
	if [ "$dollar_out" = '$HOME' ]; then
		pass_or_fail ok "a dollar sign in an argument is treated as data"
	else
		pass_or_fail bad "a dollar sign in an argument is treated as data (got '$dollar_out')"
	fi

	# Secret handling: a credential-bearing variable is not forwarded into the unit.
	local env_out
	env_out="$(GOPROXY='https://user:secret@proxy.invalid' MEMCHECK_SECRET='topsecret' \
		timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	if printf '%s' "$env_out" | grep -qE 'GOPROXY|MEMCHECK_SECRET|topsecret'; then
		pass_or_fail bad "a credential-bearing variable is not forwarded into the unit"
	elif printf '%s' "$env_out" | grep -q '^PATH='; then
		pass_or_fail ok "a credential-bearing variable is not forwarded into the unit"
	else
		pass_or_fail bad "the environment allowlist did not forward PATH into the unit"
	fi

	# Secret handling: a passwordless token URL is also skipped, so a token never
	# reaches the systemd-run command line.
	local token_out
	token_out="$(GOPROXY='https://tokenonly@proxy.invalid' \
		timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	if printf '%s' "$token_out" | grep -qE 'GOPROXY|tokenonly'; then
		pass_or_fail bad "a passwordless token URL is not forwarded into the unit"
	else
		pass_or_fail ok "a passwordless token URL is not forwarded into the unit"
	fi

	# Secret handling: a secret in the per-user manager environment is not inherited
	# by the workload, because the supervisor builds the environment from scratch.
	local mgr_out
	systemctl --user set-environment MEMCHECK_MGR_SECRET=managertopsecret >/dev/null 2>&1 || true
	mgr_out="$(timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	systemctl --user unset-environment MEMCHECK_MGR_SECRET >/dev/null 2>&1 || true
	if printf '%s' "$mgr_out" | grep -qE 'MEMCHECK_MGR_SECRET|managertopsecret'; then
		pass_or_fail bad "a manager-environment secret is not inherited by the unit"
	else
		pass_or_fail ok "a manager-environment secret is not inherited by the unit"
	fi

	# Transparency: a safe GREMLINS_* configuration value survives into the unit, so
	# a bounded run honors the same settings as a direct Gremlins command.
	local safe_out
	safe_out="$(GREMLINS_MEMCHECK_MARKER=preserved \
		timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	if printf '%s' "$safe_out" | grep -q '^GREMLINS_MEMCHECK_MARKER=preserved$'; then
		pass_or_fail ok "a safe GREMLINS_* setting survives into the unit"
	else
		pass_or_fail bad "a safe GREMLINS_* setting survives into the unit"
	fi

	# Transparency: the caller's XDG_CONFIG_HOME selection survives, so the bounded
	# run finds the same Gremlins configuration location as a direct command.
	local xdg_out
	xdg_out="$(XDG_CONFIG_HOME=/tmp/memcheck-xdg \
		timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	if printf '%s' "$xdg_out" | grep -q '^XDG_CONFIG_HOME=/tmp/memcheck-xdg$'; then
		pass_or_fail ok "the caller's XDG_CONFIG_HOME survives into the unit"
	else
		pass_or_fail bad "the caller's XDG_CONFIG_HOME survives into the unit"
	fi

	# Temp handling: the supervisor sets TMPDIR under its own root for cleanup but
	# does not clobber GOTMPDIR, which the go command and Gremlins manage themselves.
	local tmp_out
	tmp_out="$(timeout 90 "$RUN_BOUNDED" --budget "$FIT_BUDGET" "${sl[@]}" -- /usr/bin/env 2>/dev/null || true)"
	if printf '%s' "$tmp_out" | grep -q '^GOTMPDIR='; then
		pass_or_fail bad "the supervisor does not clobber GOTMPDIR"
	elif printf '%s' "$tmp_out" | grep -qE '^TMPDIR=.*run-bounded-'; then
		pass_or_fail ok "the supervisor sets TMPDIR under its own root and leaves GOTMPDIR alone"
	else
		pass_or_fail bad "the supervisor did not set TMPDIR under its own root"
	fi

	# Budget validation: a zero, infinite, excessive, or overflow budget is rejected.
	check 1 "zero budget is rejected" "$RUN_BOUNDED" --budget 0 -- /bin/true
	check 1 "infinite budget is rejected" "$RUN_BOUNDED" --budget infinity -- /bin/true
	check 1 "a budget above host memory is rejected" "$RUN_BOUNDED" --budget 1024T -- /bin/true
	check 1 "an overflow-boundary budget is rejected" \
		"$RUN_BOUNDED" --budget 9223372036854775808 -- /bin/true

	# Fail-closed: no usable manager means no run, never an unbounded run.
	check 1 "an unavailable user manager fails closed" \
		env -u DBUS_SESSION_BUS_ADDRESS XDG_RUNTIME_DIR=/nonexistent-run-bounded \
		"$RUN_BOUNDED" --budget "$OOM_BUDGET" -- /bin/true

	# Fail-closed: the runner refuses to execute with effective UID 0. Establish a
	# real root context with a user namespace, or fakeroot, so the check reflects the
	# actual effective UID rather than a PATH-resolved helper.
	local root_prefix=()
	if unshare -r true >/dev/null 2>&1; then
		root_prefix=(unshare -r)
	elif command -v fakeroot >/dev/null 2>&1; then
		root_prefix=(fakeroot)
	fi
	if [ ${#root_prefix[@]} -eq 0 ]; then
		pass_or_fail bad "no user namespace or fakeroot to establish a root context for the root-guard case"
	else
		check 1 "a root invocation is rejected" \
			"${root_prefix[@]}" "$RUN_BOUNDED" --budget "$OOM_BUDGET" -- /bin/true
	fi

	# Cancellation: a signal to the supervisor stops the workload unit.
	cancel_case

	if [ "$fail" != 0 ]; then
		note "RESULT: FAIL"
		exit 1
	fi
	note "RESULT: PASS"
}

main "$@"
