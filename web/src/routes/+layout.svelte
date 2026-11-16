<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner } from '#lib/ui/index.js';
	import { system } from '#lib/api.js';
	import { buildVersion } from '#lib/version.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	let serverVersion = $state('');

	onMount(() => {
		system
			.getVersion({})
			.then((res) => (serverVersion = res.version))
			.catch(() => {});
	});
</script>

<svelte:head>
	<title>{m.app_title()}</title>
</svelte:head>

<div class="miso-app">
	{#if serverVersion && serverVersion !== buildVersion}
		<Banner tone="info">{m.version_mismatch({ ui: buildVersion, server: serverVersion })}</Banner>
	{/if}
	{@render children()}
</div>
