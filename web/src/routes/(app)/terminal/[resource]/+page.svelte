<script lang="ts">
	import { page } from '$app/state';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import StreamView from '#lib/terminal/StreamView.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const resource = $derived(page.params.resource ?? '');
</script>

<PageHeader
	title={m.terminal_title({ resource })}
	breadcrumbs={[{ label: m.resources_title(), href: '/resources' }, { label: resource }]}
/>
<PageBody>
	{#key resource}
		<StreamView interactive path="/api/terminal" params={{ resource }} />
	{/key}
</PageBody>
