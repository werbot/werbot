<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { page } from '$app/state';
	import { Badge, Banner, Button, InfoBlock, Skeleton, Tabs } from '#lib/ui/index.js';
	import { gateways, resources } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
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
	import { firstPort, launchHint } from '#lib/resources/presets.js';
	import { hasTerminal, terminalHref } from '#lib/terminal/socket.js';
	import { m } from '#lib/paraglide/messages.js';

	let resource = $state<Resource>();
	let types = $state<ResourceType[]>([]);
	let pool = $state<Gateway>();
	let failed = $state(false);
	let tab = $state('settings');

	const tabOptions = $derived([
		{ value: 'settings', label: m.resources_tab_settings() },
		{ value: 'credentials', label: m.resources_tab_credentials() },
		{ value: 'rules', label: m.resources_tab_rules() },
		{ value: 'access', label: m.resources_tab_access() },
		{ value: 'host_key', label: m.resources_tab_host_key() }
	]);

	const preset = $derived(resource?.presetId ? types.find((t) => t.presetId === resource?.presetId) : undefined);
	const presetHint = $derived.by(() => {
		if (!preset?.launchHint || !resource) return '';
		return launchHint(preset.launchHint, '127.0.0.1', firstPort(resource.configJson) || preset.defaultPorts[0]?.port || 0);
	});
	// a pool with only a WebSocket URL has no ssh host to show
	const wsOnly = $derived(!!pool?.wsUrl && !pool.sshAddress);

	// Connect hints by connector kinds: terminal or exec sessions, stream forwarding, datagram access.
	const hints = $derived.by(() => {
		const kinds = types.find((t) => t.name === resource?.type)?.kinds ?? [];
		const args = { login: page.data.session?.user?.login ?? '', resource: resource?.name ?? '' };
		// an ssh resource without port forwarding refuses every stream
		const ssh = resource?.config && anyIs(resource.config, SshConfigSchema) ? anyUnpack(resource.config, SshConfigSchema) : undefined;
		const stream = kinds.includes('stream') && (!ssh || ssh.portForwarding);
		const out: string[] = [];
		if (wsOnly) {
			if (kinds.includes('terminal') || kinds.includes('exec')) out.push(m.resources_connect_hint_ws_ssh(args));
			if (stream || kinds.includes('datagram')) out.push(m.resources_connect_hint_ws_connect(args));
			return out;
		}
		if (kinds.includes('terminal') || kinds.includes('exec')) out.push(m.resources_connect_hint_terminal(args));
		if (stream) out.push(m.resources_connect_hint_stream(args));
		if (stream || kinds.includes('datagram')) out.push(m.resources_connect_hint_connect(args));
		return out;
	});

	async function load() {
		failed = false;
		try {
			[{ resource }, { types }] = await Promise.all([
				resources.getResource({ id: page.params.id }),
				resources.listResourceTypes({})
			]);
			// only the hints need the pool: a failed lookup keeps the default hints
			pool = (await gateways.listGateways({}).catch(() => undefined))?.gateways.find((g) => g.id === resource?.gatewayId);
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	onMount(load);
</script>

<PageHeader title={resource?.name ?? ''} breadcrumbs={[{ label: m.resources_title(), href: '/resources' }, { label: resource?.name ?? '' }]}>
	{#snippet toolbar()}
		{#if resource}<HealthDot health={resource.health} />{/if}
		{#if preset}<Badge tone="accent">{m.resources_preset_badge({ name: preset.displayName || preset.name })}</Badge>{/if}
		{#if resource && hasTerminal(types, resource.type)}
			<Button scheme="g" variant="plain" size={40} onclick={() => goto(terminalHref(resource!.name))}>{m.terminal_open()}</Button>
		{/if}
	{/snippet}
	{#snippet tabs()}
		<Tabs options={tabOptions} bind:value={tab} />
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !resource}
		<Skeleton variant="rect" height={240} />
	{:else if resource}
		{#if resource.health?.error}<Banner tone="alert">{m.resources_health_error({ error: resource.health.error })}</Banner>{/if}
		{#if hints.length || presetHint}
			<InfoBlock title={m.resources_connect_title()}>
				{#each hints as h (h)}<div><code>{h}</code></div>{/each}
				{#if presetHint}<div>{m.resources_launch_hint()}: <code>{presetHint}</code></div>{/if}
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
