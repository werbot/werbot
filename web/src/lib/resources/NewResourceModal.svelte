<script lang="ts">
	import { goto } from '$app/navigation';
	import { Banner, Modal, Spinner, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import ResourceForm, { type ResourceValues } from '#lib/resources/ResourceForm.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	let types = $state<ResourceType[]>();
	let error = $state('');

	async function create(v: ResourceValues) {
		error = '';
		try {
			const { resource } = await resources.createResource(v);
			toast({ tone: 'success', text: m.common_saved() });
			open = false;
			await goto('/resources/' + resource?.id);
		} catch (e) {
			error = errorMessage(e);
		}
	}

	// types load on the first open
	$effect(() => {
		if (!open || types) return;
		error = '';
		resources.listResourceTypes({}).then(
			(r) => (types = r.types),
			(e) => (error = errorMessage(e))
		);
	});
</script>

<Modal bind:open plain title={m.resources_new()} width={720}>
	<div class="body">
		{#if error}<Banner tone="alert">{error}</Banner>{/if}
		{#if types}<ResourceForm {types} onsubmit={create} />{:else if !error}<Spinner />{/if}
	</div>
</Modal>

<style>
	.body {
		padding: var(--miso-sp-12) 28px;
	}
</style>
