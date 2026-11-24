<script lang="ts">
	import { Code, ConnectError } from '@connectrpc/connect';
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Button, EmptyState, PropertyRow, Skeleton } from '#lib/ui/index.js';
	import { mfaChallenges } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { GetChallengeResponse } from '#lib/gen/werbot/v1/mfa_pb.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { countdown, secondsLeft } from '#lib/mfa/challenge.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { failPasskey, getAssertion } from '#lib/webauthn.js';
	import { m } from '#lib/paraglide/messages.js';

	const id = $derived(page.params.id ?? '');

	type View = 'loading' | 'ready' | 'missing' | 'closed' | 'failed' | 'confirmed';
	let view = $state<View>('loading');
	let challenge = $state<GetChallengeResponse>();
	let now = $state(Date.now());

	const left = $derived(secondsLeft(challenge?.expiresAt, now));
	const expired = $derived(view === 'ready' && left === 0);

	async function load() {
		view = 'loading';
		try {
			challenge = await mfaChallenges.getChallenge({ id });
			view = 'ready';
		} catch (e) {
			const code = ConnectError.from(e).code;
			if (code === Code.NotFound) view = 'missing';
			else if (code === Code.FailedPrecondition) view = 'closed';
			else {
				view = 'failed';
				failToast(e);
			}
		}
	}

	async function confirm() {
		if (!challenge) return;
		try {
			await mfaChallenges.confirmChallenge({ id, webauthnAssertion: await getAssertion(challenge.webauthnOptions) });
			view = 'confirmed';
		} catch (e) {
			failPasskey(e);
		}
	}

	onMount(() => {
		void load();
		const timer = setInterval(() => (now = Date.now()), 1000);
		return () => clearInterval(timer);
	});
</script>

<PageHeader title={m.mfa_challenge_title()} />
<PageBody width={520}>
	{#if view === 'loading'}
		<Skeleton variant="text" lines={4} />
	{:else if view === 'failed'}
		<LoadFailed onretry={load} />
	{:else if view === 'missing'}
		<EmptyState icon="lock" title={m.mfa_challenge_not_found()} />
	{:else if view === 'closed'}
		<EmptyState icon="lock" title={m.mfa_challenge_closed()} />
	{:else if view === 'confirmed'}
		<EmptyState icon="circle-check" title={m.mfa_challenge_confirmed()}>{m.mfa_challenge_confirmed_hint()}</EmptyState>
	{:else if challenge}
		<p class="check">{m.mfa_challenge_check()}</p>
		<div class="code" aria-label={m.mfa_challenge_code()}>{challenge.code}</div>
		<div class="props">
			<PropertyRow label={m.mfa_challenge_host()}>{challenge.host}</PropertyRow>
			<PropertyRow label={m.mfa_challenge_ip()}>{challenge.clientIp || '—'}</PropertyRow>
			<PropertyRow label={m.mfa_challenge_resource()}>{challenge.resource || '—'}</PropertyRow>
			<PropertyRow label={m.mfa_challenge_expires()}>{expired ? m.mfa_challenge_expired() : countdown(left)}</PropertyRow>
		</div>
		{#if challenge.webauthnOptions.length > 0}
			<Button variant="solid" size={40} iconLeft="key" disabled={expired} onclick={confirm}>{m.mfa_challenge_confirm()}</Button>
		{:else}
			<p class="hint">{m.mfa_challenge_totp_hint()}</p>
		{/if}
	{/if}
</PageBody>

<style>
	p {
		margin: 0;
	}

	.check {
		font: var(--miso-font-ui-14-med);
	}

	.code {
		font: 700 40px/48px var(--miso-font-mono, monospace);
		letter-spacing: 0.12em;
		text-align: center;
		padding: var(--miso-sp-16) 0;
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-mid);
		user-select: all;
	}

	.props {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
	}

	.hint {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}
</style>
