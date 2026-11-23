<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { sessions } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import StreamView from '#lib/terminal/StreamView.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const id = $derived(page.params.id ?? '');
	let name = $state('');

	onMount(() => {
		sessions.getSession({ id: id }).then((r) => (name = r.session?.resourceName ?? ''), failToast);
	});
</script>

<PageHeader
	title={m.live_title({ resource: name || id })}
	breadcrumbs={[
		{ label: m.sessions_title(), href: '/sessions' },
		{ label: name || id, href: `/sessions/${id}` },
		{ label: m.sessions_watch() }
	]}
/>
<PageBody>
	{#key id}
		<StreamView path={`/api/sessions/${id}/live`} />
	{/key}
</PageBody>
