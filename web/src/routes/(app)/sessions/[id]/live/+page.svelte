<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { sessions } from '#lib/api.js';
	import { Button } from '#lib/ui/index.js';
	import { canControlSession } from '#lib/sessions/access.js';
	import TerminateDrawer from '#lib/sessions/TerminateDrawer.svelte';
	import { failToast } from '#lib/errors.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import StreamView from '#lib/terminal/StreamView.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const id = $derived(page.params.id ?? '');
	let name = $state('');
	let owner = $state('');
	let terminating = $state(false);

	const control = $derived(canControlSession(page.data.session?.role, page.data.session?.user?.login, owner));

	onMount(() => {
		sessions.getSession({ id: id }).then((r) => ((name = r.session?.resourceName ?? ''), (owner = r.session?.userLogin ?? '')), failToast);
	});
</script>

<PageHeader
	title={m.live_title({ resource: name || id })}
	breadcrumbs={[
		{ label: m.sessions_title(), href: '/sessions' },
		{ label: name || id, href: `/sessions/${id}` },
		{ label: m.sessions_watch() }
	]}
>
	{#snippet toolbar()}
		{#if control}
			<Button scheme="g" variant="plain" size={40} iconLeft="close" onclick={() => (terminating = true)}>{m.live_terminate()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<TerminateDrawer id={terminating ? id : null} onclose={() => (terminating = false)} />
<PageBody>
	{#key id}
		<StreamView path={`/api/sessions/${id}/live`} />
	{/key}
</PageBody>
