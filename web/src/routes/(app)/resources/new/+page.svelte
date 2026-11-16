<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Banner, Breadcrumbs, toast } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import ResourceForm, { type ResourceValues } from '#lib/resources/ResourceForm.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let types = $state<ResourceType[]>();
	let error = $state('');

	async function create(v: ResourceValues) {
		error = '';
		try {
			const { resource } = await resources.createResource(v);
			toast({ tone: 'success', text: m.common_saved() });
			await goto('/resources/' + resource?.id);
		} catch (e) {
			error = errorMessage(e);
		}
	}

	onMount(async () => {
		try {
			({ types } = await resources.listResourceTypes({}));
		} catch (e) {
			error = errorMessage(e);
		}
	});
</script>

<Breadcrumbs items={[{ label: m.resources_title(), href: '/resources' }, { label: m.resources_new() }]} />
<h1>{m.resources_new()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
{#if types}<ResourceForm {types} onsubmit={create} />{/if}
