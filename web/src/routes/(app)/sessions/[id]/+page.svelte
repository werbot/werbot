<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Banner, Breadcrumbs, PropertyRow } from '#lib/ui/index.js';
	import { sessions } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { fmt } from '#lib/gateways/format.js';
	import CastPlayer from '#lib/player/CastPlayer.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let session = $state<Session>();
	let error = $state('');
	let verified = $state<boolean>();

	const finished = $derived(!!session?.recording?.finished);

	onMount(async () => {
		try {
			({ session } = await sessions.getSession({ id: page.params.id }));
		} catch (e) {
			error = errorMessage(e);
		}
	});
</script>

<Breadcrumbs items={[{ label: m.sessions_title(), href: '/sessions' }, { label: session?.resourceName ?? '' }]} />
{#if error}<Banner tone="alert">{error}</Banner>{/if}
{#if session}
	<h1>{session.resourceName || session.id}</h1>
	<PropertyRow label={m.sessions_col_user()}>{session.userLogin || '—'}</PropertyRow>
	<PropertyRow label={m.sessions_col_resource()}>{session.resourceName || '—'}</PropertyRow>
	<PropertyRow label={m.sessions_client_ip()}>{session.clientIp || '—'}</PropertyRow>
	<PropertyRow label={m.sessions_col_started()}>{fmt(session.startedAt)}</PropertyRow>
	<PropertyRow label={m.sessions_col_ended()}>{fmt(session.endedAt)}</PropertyRow>
	<PropertyRow label={m.sessions_end_reason()}>{session.endReason || '—'}</PropertyRow>

	{#if !finished}
		<Banner tone="info">{m.sessions_recording_pending()}</Banner>
	{:else if verified === true}
		<Banner tone="success">{m.sessions_recording_verified()}</Banner>
	{:else if verified === false}
		<Banner tone="alert">{m.sessions_recording_failed()}</Banner>
	{/if}
	{#if session.recording?.chunks}
		<CastPlayer src={`/api/recordings/${session.id}.cast`} onverified={(v) => (verified = v)} />
	{/if}
{/if}
