<script lang="ts">
	import { onMount } from 'svelte';
	import { EmptyState } from '#lib/ui/index.js';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';

	let version = $state('');
	let failed = $state(false);

	onMount(() => {
		system
			.getVersion({})
			.then((res) => (version = res.version))
			.catch(() => (failed = true));
	});
</script>

<EmptyState title={m.app_title()}>
	{#if failed}
		{m.version_unavailable()}
	{:else if version}
		{m.version_label({ version })}
	{/if}
</EmptyState>
