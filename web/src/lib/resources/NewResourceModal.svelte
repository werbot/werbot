<script lang="ts">
	import { goto } from '$app/navigation';
	import { Banner, Breadcrumbs, Button, Modal, Spinner, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import ResourceForm, { type ResourceValues } from '#lib/resources/ResourceForm.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { categoryIcon, categoryLabel, groupTypes, typeIcon } from './icons.js';
	import TileGrid from './TileGrid.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	let types = $state<ResourceType[]>();
	let error = $state('');
	let category = $state('');
	let typeName = $state('');

	let groups = $derived(groupTypes(types ?? []));
	let step = $derived(typeName ? 3 : category ? 2 : 1);
	let crumbs = $derived(
		[m.resources_step_category(), categoryLabel(category), typeName].slice(0, step).map((label) => ({ label }))
	);

	function back() {
		if (typeName) typeName = '';
		else category = '';
	}

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
		if (!open) {
			category = typeName = '';
			return;
		}
		if (types) return;
		error = '';
		resources.listResourceTypes({}).then(
			(r) => (types = r.types),
			(e) => (error = errorMessage(e))
		);
	});
</script>

<Modal bind:open plain title={m.resources_new()} width={720}>
	<div class={['body', step === 3 && 'body--form']}>
		{#if error}<Banner tone="alert">{error}</Banner>{/if}
		{#if types}
			<div class="steps">
				{#if step === 2}<Button scheme="g" variant="plain" iconLeft="arrows/chevron-left" onclick={back}>{m.resources_back()}</Button>{/if}
				<Breadcrumbs items={crumbs} />
			</div>
			{#if step === 1}
				<TileGrid
					label={m.resources_step_category()}
					tiles={groups.map((g) => ({ value: g.category, label: categoryLabel(g.category), icon: categoryIcon(g.category) }))}
					onpick={(v) => (category = v)}
				/>
			{:else if step === 2}
				<TileGrid
					label={m.resources_step_type()}
					tiles={(groups.find((g) => g.category === category)?.types ?? []).map((t) => ({ value: t.name, label: t.name, icon: typeIcon(t.name) }))}
					onpick={(v) => (typeName = v)}
				/>
			{:else}
				<ResourceForm {types} {typeName} onsubmit={create} onback={back} />
			{/if}
		{:else if !error}<Spinner />{/if}
	</div>
</Modal>

<style>
	.body {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		/* 28px, the modal's own side padding: the same on the bottom edge */
		--pad: calc(var(--miso-sp-16) + var(--miso-sp-12));
		padding: var(--miso-sp-12) var(--pad) var(--pad);
	}

	/* step 3: the sticky footer carries the bottom padding */
	.body--form {
		padding-bottom: 0;
	}

	.steps {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-8);
	}
</style>
