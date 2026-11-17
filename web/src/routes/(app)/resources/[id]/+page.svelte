<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banner, Breadcrumbs, Tabs } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import AccessTab from '#lib/resources/AccessTab.svelte';
	import CredentialsTab from '#lib/resources/CredentialsTab.svelte';
	import HostKeyTab from '#lib/resources/HostKeyTab.svelte';
	import RulesTab from '#lib/resources/RulesTab.svelte';
	import SettingsTab from '#lib/resources/SettingsTab.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let resource = $state<Resource>();
	let types = $state<ResourceType[]>([]);
	let error = $state('');
	let tab = $state('settings');

	const tabs = $derived([
		{ value: 'settings', label: m.resources_tab_settings() },
		{ value: 'credentials', label: m.resources_tab_credentials() },
		{ value: 'rules', label: m.resources_tab_rules() },
		{ value: 'access', label: m.resources_tab_access() },
		{ value: 'host_key', label: m.resources_tab_host_key() }
	]);

	// Connect hints by connector kinds: terminal or exec sessions, stream forwarding, datagram access.
	const hints = $derived.by(() => {
		const kinds = types.find((t) => t.name === resource?.type)?.kinds ?? [];
		const args = { login: page.data.session?.user?.login ?? '', resource: resource?.name ?? '' };
		const out: string[] = [];
		if (kinds.includes('terminal') || kinds.includes('exec')) out.push(m.resources_connect_hint_terminal(args));
		if (kinds.includes('stream')) out.push(m.resources_connect_hint_stream(args));
		if (kinds.includes('stream') || kinds.includes('datagram')) out.push(m.resources_connect_hint_connect(args));
		return out;
	});

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
	{#if resource.health?.error}<Banner tone="alert">{m.resources_health_error({ error: resource.health.error })}</Banner>{/if}
	{#each hints as h (h)}<p><code>{h}</code></p>{/each}
	<Tabs options={tabs} bind:value={tab} />
	{#if tab === 'settings'}
		<SettingsTab bind:resource {types} />
	{:else if tab === 'credentials'}
		<CredentialsTab bind:resource type={types.find((t) => t.name === resource?.type)} />
	{:else if tab === 'rules'}
		<RulesTab resourceId={resource.id} />
	{:else if tab === 'access'}
		<AccessTab {resource} />
	{:else}
		<HostKeyTab bind:resource />
	{/if}
{/if}
