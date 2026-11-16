<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banner, Breadcrumbs, EmptyState, Tabs } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import SettingsTab from '#lib/resources/SettingsTab.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let resource = $state<Resource>();
	let types = $state<ResourceType[]>([]);
	let error = $state('');
	let tab = $state('settings');

	const tabs = [
		{ value: 'settings', label: m.resources_tab_settings() },
		{ value: 'credentials', label: m.resources_tab_credentials() },
		{ value: 'rules', label: m.resources_tab_rules() },
		{ value: 'access', label: m.resources_tab_access() },
		{ value: 'host_key', label: m.resources_tab_host_key() }
	];

	onMount(async () => {
		try {
			[{ resource }, { types }] = await Promise.all([
				resources.getResource({ id: page.params.id }),
				resources.listResourceTypes({})
			]);
		} catch (e) {
			error = errorMessage(e);
		}
	});
</script>

<Breadcrumbs items={[{ label: m.resources_title(), href: '/resources' }, { label: resource?.name ?? '' }]} />
{#if error}<Banner tone="alert">{error}</Banner>{/if}
{#if resource}
	<h1>{resource.name}</h1>
	<Tabs options={tabs} bind:value={tab} />
	{#if tab === 'settings'}
		<SettingsTab bind:resource {types} />
	{:else}
		<EmptyState title={m.resources_tab_pending()} />
	{/if}
{/if}
