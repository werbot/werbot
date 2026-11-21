<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Banner, Skeleton } from '#lib/ui/index.js';
	import ClientCard from '#lib/cli/ClientCard.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { oauthConsent } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { GetAuthRequestResponse } from '#lib/gen/werbot/v1/oauth_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	const id = page.url.searchParams.get('id') ?? '';
	let client = $state<GetAuthRequestResponse>();
	let expired = $state(!id);
	let failed = $state(false);
	let busy = $state(false);

	// NotFound: the request expired, was answered, or the id is not a request
	function fail(e: unknown) {
		const code = ConnectError.from(e).code;
		if (code === Code.NotFound || code === Code.InvalidArgument) expired = true;
		else failToast(e);
	}

	async function load() {
		failed = false;
		try {
			client = await oauthConsent.getAuthRequest({ id });
		} catch (e) {
			fail(e);
			failed = !expired;
		}
	}

	// both answers hand the browser to op's callback, which redirects to the CLI
	async function decide(approve: boolean) {
		busy = true;
		try {
			const { redirectUrl } = approve ? await oauthConsent.approveAuthRequest({ id }) : await oauthConsent.denyAuthRequest({ id });
			location.assign(redirectUrl);
		} catch (e) {
			fail(e);
			busy = false;
		}
	}

	onMount(() => {
		if (id) load();
	});
</script>

<PageHeader title={m.cli_authorize_title()} />
<PageBody width={680}>
	{#if expired}
		<Banner tone="info">{m.cli_authorize_expired()}</Banner>
	{:else if failed}
		<LoadFailed onretry={load} />
	{:else if !client}
		<Skeleton variant="text" lines={4} />
	{:else}
		<ClientCard
			{client}
			hint={m.cli_authorize_hint}
			approve={m.cli_authorize_approve()}
			deny={m.cli_authorize_deny()}
			{busy}
			onapprove={() => decide(true)}
			ondeny={() => decide(false)}
		/>
	{/if}
</PageBody>
