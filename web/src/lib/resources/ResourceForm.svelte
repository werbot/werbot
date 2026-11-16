<script lang="ts" module>
	import type { ConfigValue } from './forms/index.js';

	export type ResourceValues = ConfigValue & { name: string; type: string; labels: Record<string, string> };
</script>

<script lang="ts">
	import { Button, Select, TextField } from '#lib/ui/index.js';
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
	let value = $state.raw<ConfigValue>({});

	let type = $derived(types.find((t) => t.name === typeName));
	let valid = $derived(!!value.config || value.configJson !== undefined);
	let Form = $derived(configFormFor(typeName));

	function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!valid) return;
		onsubmit({ ...value, name, type: typeName, labels: labelsFromRows(rows) });
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
	{#if type}
		{#key typeName}
			<Form {type} {initial} onchange={(v) => (value = v)} />
		{/key}
	{/if}
	<LabelsEditor bind:rows />
	<div><Button type="submit" variant="solid" disabled={!valid}>{m.resources_save()}</Button></div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 480px;
	}
</style>
