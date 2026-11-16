<script lang="ts">
	import { goto } from '$app/navigation';
	import { Banner, Button, Modal, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import ResourceForm, { type ResourceValues } from './ResourceForm.svelte';

	let { resource = $bindable(), types }: { resource: Resource; types: ResourceType[] } = $props();

	let error = $state('');
	let confirming = $state(false);

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: m.common_saved() });
			return true;
		} catch (e) {
			error = errorMessage(e);
			confirming = false;
			return false;
		}
	}

	async function save({ name, labels, config, configJson }: ResourceValues) {
		await run(async () => {
			const res = await resources.updateResource({ id: resource.id, name, labels, config, configJson });
			if (res.resource) resource = res.resource;
		});
	}

	async function remove() {
		if (await run(() => resources.deleteResource({ id: resource.id }))) await goto('/resources');
	}
</script>

{#if error}<Banner tone="alert">{error}</Banner>{/if}
<ResourceForm {types} initial={resource} onsubmit={save} />
<p><Button scheme="w" variant="plain" onclick={() => (confirming = true)}>{m.resources_delete()}</Button></p>

<Modal bind:open={confirming} title={m.resources_delete()}>
	{m.resources_delete_confirm()}
	{#snippet footer()}
		<Button variant="plain" onclick={() => (confirming = false)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={remove}>{m.resources_delete()}</Button>
	{/snippet}
</Modal>
