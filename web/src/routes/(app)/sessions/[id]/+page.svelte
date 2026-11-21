<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banner, Avatar, Label, PropertyRow, Skeleton, Spinner } from '#lib/ui/index.js';
	import { sessions } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { bytes, fmt } from '#lib/format.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import CastPlayer from '#lib/player/CastPlayer.svelte';
	import { isMe } from '#lib/me.js';
	import { m } from '#lib/paraglide/messages.js';

	let session = $state<Session>();
	let failed = $state(false);
	let verified = $state<boolean>();
	let playerFailed = $state(false);

	const finished = $derived(!!session?.recording?.finished);

	async function load() {
		failed = false;
		try {
			({ session } = await sessions.getSession({ id: page.params.id }));
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	onMount(load);
</script>

<PageHeader
	title={session ? session.resourceName || session.id : ''}
	breadcrumbs={[{ label: m.sessions_title(), href: '/sessions' }, { label: session ? session.resourceName || session.id : '' }]}
/>
<PageBody>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !session}
		<Skeleton variant="rect" height={240} />
	{:else if session}
		<div class="detail">
			<div class="detail__main">
				{#if !session.recording?.chunks && session.endedAt}
					<Banner tone="info">{m.sessions_recording_none()}</Banner>
				{:else if !finished}
					<Banner tone="info">{m.sessions_recording_pending()}</Banner>
				{:else if verified === true}
					<Banner tone="success">{m.sessions_recording_verified()}</Banner>
				{:else if verified === false}
					<Banner tone="alert">{m.sessions_recording_failed()}</Banner>
				{/if}
				{#if session.recording?.chunks && finished && verified === undefined && !playerFailed}<Spinner />{/if}
				{#if session.recording?.chunks}
					<CastPlayer src={`/api/recordings/${session.id}.cast`} onverified={(v) => (verified = v)} onerror={() => (playerFailed = true)} onretry={() => (playerFailed = false)} />
				{/if}
			</div>
			<aside class="detail__props">
				<PropertyRow label={m.sessions_col_user()}>
					{#if session.userLogin}<span class={['user', isMe(session.userLogin) && 'wb-me']}><Avatar name={session.userLogin} size={24} />{session.userLogin}</span>{:else}—{/if}
				</PropertyRow>
				<PropertyRow label={m.sessions_col_resource()}>{session.resourceName || '—'}</PropertyRow>
				{#if session.kind}<PropertyRow label={m.sessions_kind()}>{session.kind}</PropertyRow>{/if}
				{#if session.target}<PropertyRow label={m.sessions_target()}>{session.target}</PropertyRow>{/if}
				{#if session.command}<PropertyRow label={m.sessions_col_command()}><code>{session.command}</code></PropertyRow>{/if}
				{#if session.classes.length > 0}<PropertyRow label={m.sessions_classes()}><ClassChips classes={session.classes} /></PropertyRow>{/if}
				{#if session.paths.length > 0}<PropertyRow label={m.sessions_paths()}><span class="paths">{#each session.paths as p (p)}<Label variant="soft"><code>{p}</code></Label>{/each}</span></PropertyRow>{/if}
				<PropertyRow label={m.sessions_client_ip()}>{session.clientIp || '—'}</PropertyRow>
				<PropertyRow label={m.sessions_col_started()}>{fmt(session.startedAt)}</PropertyRow>
				<PropertyRow label={m.sessions_col_ended()}>{fmt(session.endedAt)}</PropertyRow>
				<PropertyRow label={m.sessions_end_reason()}>{session.endReason || '—'}</PropertyRow>
				<PropertyRow label={m.sessions_bytes_up()}>{bytes(session.bytesUp)}</PropertyRow>
				<PropertyRow label={m.sessions_bytes_down()}>{bytes(session.bytesDown)}</PropertyRow>
				{#if session.kind === 'datagram'}
					<PropertyRow label={m.sessions_datagrams_up()}>{session.datagramsUp}</PropertyRow>
					<PropertyRow label={m.sessions_datagrams_down()}>{session.datagramsDown}</PropertyRow>
				{/if}
			</aside>
		</div>
	{/if}
</PageBody>

<style>
	.detail {
		display: grid;
		grid-template-columns: 1fr 320px;
		gap: var(--miso-sp-16);
		align-items: start;
	}

	code {
		font-family: var(--miso-font-mono);
		font-size: 12px;
		word-break: break-all;
	}

	.paths {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}

	.user {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
	}

	/* status banner, spinner and player stack with a gap */
	.detail__main {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		min-width: 0;
	}

	.detail__props {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-08);
		padding: var(--miso-sp-16);
	}
</style>
