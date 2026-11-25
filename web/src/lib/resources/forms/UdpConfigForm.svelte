<script lang="ts">
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { FilledField } from '#lib/ui/index.js';
	import { ConfigSchema } from '#lib/gen/werbot/connector/udp/v1/udp_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FormField from '#lib/FormField.svelte';
	import FieldRow from '#lib/FieldRow.svelte';
	import { udpConfigValue, type ConfigFormProps } from './index.js';
	import PortRangesField from './PortRangesField.svelte';
	import { formatPorts } from './ports.js';

	let { type, initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	const current = initial?.config && anyIs(initial.config, ConfigSchema) ? anyUnpack(initial.config, ConfigSchema) : undefined;
	let host = $state(current?.host ?? '');
	// svelte-ignore state_referenced_locally
	let ports = $state(formatPorts(current?.ports ?? type.defaultPorts));
	let flowIdle = $state(current?.flowIdle ? String(current.flowIdle.seconds) : '');

	const flowError = $derived(flowIdle && !/^\d+$/.test(flowIdle) ? m.resources_flow_idle_invalid() : '');

	$effect(() => {
		try {
			onchange(udpConfigValue({ host, ports, flowIdle }));
		} catch {
			onchange({});
		}
	});
</script>

<FieldRow even>
	<FilledField label={m.resources_ssh_host()} bind:value={host} required />
	<PortRangesField label={m.resources_ports()} bind:value={ports} required />
</FieldRow>
<FieldRow even>
	<FormField label={m.resources_flow_idle()} bind:value={flowIdle} type="number" min={1} max={3600} hint={m.resources_flow_idle_hint()} error={flowError} />
</FieldRow>
