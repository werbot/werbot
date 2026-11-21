<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Banner, Button, SettingsCard } from '#lib/ui/index.js';
	import ClientCard from '#lib/cli/ClientCard.svelte';
	import FormField from '#lib/FormField.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { oauthConsent } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { LookupDeviceCodeResponse } from '#lib/gen/werbot/v1/oauth_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let userCode = $state(page.url.searchParams.get('user_code') ?? '');
	let client = $state<LookupDeviceCodeResponse>();
	let outcome = $state<'done' | 'denied'>();
	let busy = $state(false);

	// a mistyped, expired or answered code is NotFound; a malformed one InvalidArgument
	function fail(e: unknown) {
		const code = ConnectError.from(e).code;
		failToast(code === Code.NotFound || code === Code.InvalidArgument ? m.device_not_found() : e);
	}

	async function lookup(e?: SubmitEvent) {
		e?.preventDefault();
		busy = true;
		try {
			client = await oauthConsent.lookupDeviceCode({ userCode });
		} catch (err) {
			fail(err);
		}
		busy = false;
	}

	async function decide(approve: boolean) {
		busy = true;
		try {
			await (approve ? oauthConsent.approveDeviceCode({ userCode }) : oauthConsent.denyDeviceCode({ userCode }));
			outcome = approve ? 'done' : 'denied';
		} catch (e) {
			fail(e);
			client = undefined;
		}
		busy = false;
	}

	// the CLI's verification link carries the code: look it up at once, the person still approves
	onMount(() => {
		if (userCode) lookup();
	});
</script>

<PageHeader title={m.device_title()} />
<PageBody width={680}>
	{#if outcome === 'done'}
		<Banner tone="success">{m.device_done()}</Banner>
	{:else if outcome === 'denied'}
		<Banner tone="info">{m.device_denied()}</Banner>
	{:else if client}
		<ClientCard
			{client}
			hint={m.device_hint}
			approve={m.device_approve()}
			deny={m.device_deny()}
			{busy}
			onapprove={() => decide(true)}
			ondeny={() => decide(false)}
		/>
	{:else}
		<SettingsCard>
			<form onsubmit={lookup}>
				<FormField label={m.device_code_label()} bind:value={userCode} required maxlength={16} autocomplete="off" />
				<div><Button size={40} type="submit" variant="solid" disabled={busy}>{m.device_lookup()}</Button></div>
			</form>
		</SettingsCard>
	{/if}
</PageBody>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}
</style>
