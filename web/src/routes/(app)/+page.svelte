<script lang="ts">
	import { onMount } from 'svelte';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let version = $state('');
	let failed = $state(false);

	onMount(() => {
		system
			.getVersion({})
			.then((res) => (version = res.version))
			.catch(() => (failed = true));
	});
</script>

<h1>{m.dashboard_title()}</h1>
<p>{data.session.organization?.name}</p>
<p>{failed ? m.version_unavailable() : version && m.version_label({ version })}</p>
