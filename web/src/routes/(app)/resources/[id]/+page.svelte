<script lang="ts">
	import { onMount } from 'svelte';
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { page } from '$app/state';
	import { Banner, InfoBlock, Skeleton, Tabs } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
	import { ConfigSchema as SshConfigSchema } from '#lib/gen/werbot/connector/ssh/v1/ssh_pb.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import HealthDot from '#lib/resources/HealthDot.svelte';
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

	const tabOptions = $derived([
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
		// an ssh resource without port forwarding refuses every stream
		const ssh = resource?.config && anyIs(resource.config, SshConfigSchema) ? anyUnpack(resource.config, SshConfigSchema) : undefined;
		const stream = kinds.includes('stream') && (!ssh || ssh.portForwarding);
		const out: string[] = [];
		if (kinds.includes('terminal') || kinds.includes('exec')) out.push(m.resources_connect_hint_terminal(args));
		if (stream) out.push(m.resources_connect_hint_stream(args));
		if (stream || kinds.includes('datagram')) out.push(m.resources_connect_hint_connect(args));
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

<PageHeader title={resource?.name ?? ''} breadcrumbs={[{ label: m.resources_title(), href: '/resources' }, { label: resource?.name ?? '' }]}>
	{#snippet toolbar()}
		{#if resource}<HealthDot health={resource.health} />{/if}
	{/snippet}
	{#snippet tabs()}
		<Tabs options={tabOptions} bind:value={tab} />
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	{#if !resource && !error}
		<Skeleton variant="rect" height={240} />
	{:else if resource}
		{#if resource.health?.error}<Banner tone="alert">{m.resources_health_error({ error: resource.health.error })}</Banner>{/if}
		{#if hints.length}
			<InfoBlock title={m.resources_connect_title()}>
				{#each hints as h (h)}<div><code>{h}</code></div>{/each}
			</InfoBlock>
		{/if}
		<div class="card">
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
		</div>
	{/if}
</PageBody>

<style>
	.card {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-08);
		padding: var(--miso-sp-16);
	}
</style>
