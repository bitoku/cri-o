#!/usr/bin/env bats

load helpers

function setup() {
	setup_test
	if [[ $RUNTIME_TYPE == vm ]]; then
		skip "not applicable to vm runtime type"
	fi
	if ! is_cgroup_v2; then
		skip "blockio tests require cgroup v2"
	fi
}
bl
function teardown() {
	cleanup_test
}

function configure_blockio() {
	cat <<EOF >"$CRIO_CONFIG_DIR/01-blockio.conf"
[crio.runtime]
blockio_config_file = "$TESTDATA/blockio.yaml"
EOF
}

function get_ctr_cgroup_dir() {
	local ctr_id="$1"
	local pid
	pid=$(crictl inspect "$ctr_id" | jq -r '.info.pid // empty')
	[[ -n "$pid" ]]
	local cgrel
	cgrel=$(head -1 "/proc/$pid/cgroup" | cut -d: -f3)
	echo "/sys/fs/cgroup${cgrel}"
}

@test "blockio baseline without annotation has no io.max throttle entries" {
	configure_blockio
	start_crio

	ctr_id=$(crictl run "$TESTDATA"/container_sleep.json "$TESTDATA"/sandbox_config.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]

	if [[ -f "$cgdir/io.max" ]]; then
		# baseline should have no throttle entries
		run ! grep -q "rbps=" "$cgdir/io.max"
	fi
}

@test "blockio pod annotation applies io.max throttle" {
	configure_blockio
	start_crio

	jq '	  .annotations["blockio.resources.beta.kubernetes.io/pod"] = "lowprio"' \
		"$TESTDATA"/sandbox_config.json >"$TESTDIR"/sandbox_blockio.json

	ctr_id=$(crictl run "$TESTDATA"/container_sleep.json "$TESTDIR"/sandbox_blockio.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]
	[[ -f "$cgdir/io.max" ]]
	grep -q "rbps=" "$cgdir/io.max"
}

@test "blockio per-container pod annotation applies io.max throttle" {
	configure_blockio
	start_crio

	jq '	  .annotations["blockio.resources.beta.kubernetes.io/container.podsandbox-sleep"] = "highprio"' \
		"$TESTDATA"/sandbox_config.json >"$TESTDIR"/sandbox_blockio.json

	ctr_id=$(crictl run "$TESTDATA"/container_sleep.json "$TESTDIR"/sandbox_blockio.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]
	[[ -f "$cgdir/io.max" ]]
	grep -q "rbps=" "$cgdir/io.max"
}

@test "blockio CRI container annotation applies io.max throttle" {
	configure_blockio
	start_crio

	jq '	  .annotations["io.kubernetes.cri.blockio-class"] = "lowprio"' \
		"$TESTDATA"/container_sleep.json >"$TESTDIR"/container_blockio.json

	ctr_id=$(crictl run "$TESTDIR"/container_blockio.json "$TESTDATA"/sandbox_config.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]
	[[ -f "$cgdir/io.max" ]]
	grep -q "rbps=" "$cgdir/io.max"
}

@test "blockio per-container annotation overrides pod-level annotation" {
	configure_blockio
	start_crio

	jq '	  .annotations["blockio.resources.beta.kubernetes.io/pod"] = "lowprio"
		| .annotations["blockio.resources.beta.kubernetes.io/container.podsandbox-sleep"] = "highprio"' \
		"$TESTDATA"/sandbox_config.json >"$TESTDIR"/sandbox_blockio.json

	ctr_id=$(crictl run "$TESTDATA"/container_sleep.json "$TESTDIR"/sandbox_blockio.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]
	[[ -f "$cgdir/io.max" ]]
	grep -q "rbps=" "$cgdir/io.max"
}

@test "blockio CRI annotation overrides per-container pod annotation" {
	configure_blockio
	start_crio

	jq '	  .annotations["blockio.resources.beta.kubernetes.io/container.podsandbox-sleep"] = "highprio"' \
		"$TESTDATA"/sandbox_config.json >"$TESTDIR"/sandbox_blockio.json

	jq '	  .annotations["io.kubernetes.cri.blockio-class"] = "lowprio"' \
		"$TESTDATA"/container_sleep.json >"$TESTDIR"/container_blockio.json

	ctr_id=$(crictl run "$TESTDIR"/container_blockio.json "$TESTDIR"/sandbox_blockio.json)

	cgdir=$(get_ctr_cgroup_dir "$ctr_id")
	[[ -d "$cgdir" ]]
	[[ -f "$cgdir/io.max" ]]
	grep -q "rbps=" "$cgdir/io.max"
}
