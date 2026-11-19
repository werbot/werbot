<script lang="ts" module>
	import type { Duration } from '@bufbuild/protobuf/wkt';
	import type { ConfigValue } from './forms/index.js';

	export type ResourceValues = ConfigValue & {
		name: string;
		type: string;
		gatewayId: string;
		labels: Record<string, string>;
		idleTimeout?: Duration;
		maxDuration?: Duration;
	};
</script>

<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { create } from '@bufbuild/protobuf';
	import { DurationSchema } from '@bufbuild/protobuf/wkt';
	import { Banner, Button, Select, TextField } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { configFormFor } from './forms/index.js';
	import LabelsEditor from './LabelsEditor.svelte';
	import { labelsFromRows, rowsFromLabels } from './labels.js';

	interface Props {
		types: ResourceType[];
		initial?: Resource;
		onsubmit: (v: ResourceValues) => void;
	}

	let { types, initial, onsubmit }: Props = $props();

	// svelte-ignore state_referenced_locally
	let name = $state(initial?.name ?? '');
	// svelte-ignore state_referenced_locally
	let typeName = $state(initial?.type ?? types[0]?.name ?? '');
	// svelte-ignore state_referenced_locally
	let rows = $state(rowsFromLabels(initial?.labels ?? {}));
	// svelte-ignore state_referenced_locally
	let gatewayId = $state(initial?.gatewayId ?? '');
	let pools = $state<Gateway[]>([]);
	let loaded = $state(false);
	let poolError = $state('');
	let value = $state.raw<ConfigValue>({});
	// timeouts are edited in minutes; empty means the gateway default
	// svelte-ignore state_referenced_locally
	let idleMinutes = $state(minutes(initial?.idleTimeout));
	// svelte-ignore state_referenced_locally
	let maxMinutes = $state(minutes(initial?.maxDuration));

	let type = $derived(types.find((t) => t.name === typeName));
	let valid = $derived(!!value.config || value.configJson !== undefined);
	let Form = $derived(configFormFor(typeName));

	onMount(async () => {
		try {
			({ gateways: pools } = await gateways.listGateways({}));
			if (!gatewayId) gatewayId = pools[0]?.id ?? '';
		} catch (e) {
			// members may lack the role: the server then picks the pool
			if (ConnectError.from(e).code !== Code.PermissionDenied) poolError = errorMessage(e);
		}
		loaded = true;
	});

	function minutes(d?: Duration) {
		return d ? String(Number(d.seconds) / 60) : '';
	}

	const duration = (min: string) => (min ? create(DurationSchema, { seconds: BigInt(Math.round(Number(min) * 60)) }) : undefined);

	function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!valid) return;
		onsubmit({
			...value,
			name,
			type: typeName,
			gatewayId,
			labels: labelsFromRows(rows),
			idleTimeout: duration(idleMinutes),
			maxDuration: duration(maxMinutes)
		});
	}
</script>

<form onsubmit={submit}>
	<TextField label={m.resources_name()} bind:value={name} required maxlength={63} />
	{#if initial}
		<TextField label={m.resources_type()} value={initial.type} disabled />
	{:else}
		<Select
			label={m.resources_type()}
			options={types.map((t) => ({ value: t.name, label: t.name }))}
			bind:value={typeName}
		/>
	{/if}
	{#if poolError}<Banner tone="alert">{poolError}</Banner>{/if}
	{#if pools.length > 1}
		<Select label={m.resources_gateway_label()} options={pools.map((p) => ({ value: p.id, label: p.name }))} bind:value={gatewayId} />
	{/if}
	{#if type}
		{#key typeName}
			<Form {type} {initial} onchange={(v) => (value = v)} />
		{/key}
	{/if}
	<TextField label={m.resources_idle_timeout()} bind:value={idleMinutes} type="number" min={1} max={1440} hint={m.resources_timeout_hint()} />
	<TextField label={m.resources_max_duration()} bind:value={maxMinutes} type="number" min={1} max={10080} hint={m.resources_timeout_hint()} />
	<LabelsEditor bind:rows />
	<div><Button type="submit" variant="solid" disabled={!valid || !loaded}>{m.resources_save()}</Button></div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		max-width: 480px;
	}
</style>
