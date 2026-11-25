<script lang="ts">
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { FilledField } from '#lib/ui/index.js';
	import { ConfigSchema } from '#lib/gen/werbot/connector/tcp/v1/tcp_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import { tcpConfigValue, type ConfigFormProps } from './index.js';
	import PortRangesField from './PortRangesField.svelte';
	import { formatPorts } from './ports.js';

	let { type, initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	const current = initial?.config && anyIs(initial.config, ConfigSchema) ? anyUnpack(initial.config, ConfigSchema) : undefined;
	let host = $state(current?.host ?? '');
	// svelte-ignore state_referenced_locally
	let ports = $state(formatPorts(current?.ports ?? type.defaultPorts));

	$effect(() => {
		try {
			onchange(tcpConfigValue({ host, ports }));
		} catch {
			onchange({});
		}
	});
</script>

<FieldRow even>
	<FilledField label={m.resources_ssh_host()} bind:value={host} required />
	<PortRangesField label={m.resources_ports()} bind:value={ports} required />
</FieldRow>
