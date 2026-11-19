<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner } from '#lib/ui/index.js';
	import { system } from '#lib/api.js';
	import { buildVersion, provideServerInfo } from '#lib/version.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	const info = $state({ version: '', geoipAttribution: false, failed: false });
	provideServerInfo(info);

	onMount(() => {
		system
			.getVersion({})
			.then((res) => Object.assign(info, { version: res.version, geoipAttribution: res.geoipAttribution }))
			.catch(() => (info.failed = true));
	});
</script>

<svelte:head>
	<title>{m.app_title()}</title>
</svelte:head>

<div class="miso-app">
	{#if info.version && info.version !== buildVersion}
		<Banner tone="info">{m.version_mismatch({ ui: buildVersion, server: info.version })}</Banner>
	{/if}
	{@render children()}
</div>

<style>
	:global(body) {
		margin: 0;
	}
</style>
