<script lang="ts">
	import { Badge, Button, Drawer, PropertyRow, Skeleton, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { accessRequests } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration } from '#lib/duration.js';
	import { fmt } from '#lib/format.js';
	import type { AccessRequest } from '#lib/gen/werbot/v1/access_request_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import LabelChips from '#lib/resources/LabelChips.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import StatusBadge from './StatusBadge.svelte';

	interface Props {
		/** the request to show (the page's ?id=); empty closes the drawer */
		id: string;
		/** the signed-in user's id: their own pending access request can be canceled */
		me: string;
		onclose: () => void;
		/** a decision or cancel changed the request: the page reloads its lists */
		onchanged: () => void;
	}

	let { id, me, onclose, onchanged }: Props = $props();

	let request = $state<AccessRequest | null>(null);
	let comment = $state('');
	let busy = $state(false);
	// keep the last request on screen while the close animation plays
	const shown = keepLast(() => request);
	const view = $derived(id ? request : shown.value);
	const own = $derived(view?.requesterUserId === me && view?.status === 'pending' && view?.kind === 'access');

	// Approve, Deny and Cancel answer without the joined names and can_decide: read the request again
	async function reload(want: string) {
		try {
			const res = await accessRequests.getRequest({ id: want });
			if (want === id) request = res.request ?? null;
		} catch (e) {
			failToast(e);
			if (want === id) onclose();
		}
	}

	// a new id: drop the old request and load the new one (a data fetch, not derivable state)
	$effect(() => {
		request = null;
		comment = '';
		if (id) reload(id);
	});

	async function act(call: () => Promise<unknown>, message: string) {
		const target = id;
		busy = true;
		try {
			await call();
			toast({ tone: 'success', text: message });
			comment = '';
		} catch (e) {
			failToast(e);
		}
		busy = false;
		onchanged();
		await reload(target);
	}
</script>

<Drawer bind:open={() => !!id, (v) => !v && onclose()} resizable snapPoints={[480, 720]} width={480} title={view?.resourceName || m.requests_title()}>
	{#if view}
		{@const r = view}
		<PropertyRow label={m.requests_status()}>
			<StatusBadge status={r.status} />
			{#if r.breakGlass}<Badge tone="alert">{m.requests_break_glass()}</Badge>{/if}
		</PropertyRow>
		<PropertyRow label={m.requests_requester()}>{r.requesterLogin}</PropertyRow>
		<PropertyRow label={m.requests_resource()}>
			{#if r.resourceId}{r.resourceName}{:else}<LabelChips labels={r.labelSelector} />{/if}
		</PropertyRow>
		{#if r.kind === 'command'}
			<PropertyRow label={m.requests_command()}><code>{r.command}</code></PropertyRow>
			<PropertyRow label={m.policies_commands_classes()}><ClassChips classes={r.classes} /></PropertyRow>
		{:else}
			<PropertyRow label={m.requests_duration()}>{formatDuration(r.duration) || m.requests_duration_default()}</PropertyRow>
			<PropertyRow label={m.requests_reason()}>{r.reason || '—'}</PropertyRow>
		{/if}
		<PropertyRow label={m.requests_created()}>{fmt(r.createdAt)}</PropertyRow>
		{#if r.status === 'pending'}
			<PropertyRow label={m.requests_expires()}>{fmt(r.expiresAt)}</PropertyRow>
		{:else if r.decidedAt}
			<PropertyRow label={m.requests_decided()}>{fmt(r.decidedAt)}</PropertyRow>
		{/if}
		{#if r.comment}<PropertyRow label={m.requests_comment()}>{r.comment}</PropertyRow>{/if}
		{#if r.canDecide}
			<div class="comment"><FormField multiline label={m.requests_comment()} bind:value={comment} maxlength={1024} /></div>
		{/if}
	{:else}
		<Skeleton variant="text" lines={6} />
	{/if}
	{#snippet footer()}
		{#if view?.canDecide}
			{@const rid = view.id}
			<Button size={40} scheme="w" variant="solid" disabled={busy} onclick={() => act(() => accessRequests.deny({ id: rid, comment }), m.requests_denied())}>
				{m.requests_deny()}
			</Button>
			<Button size={40} variant="solid" disabled={busy} onclick={() => act(() => accessRequests.approve({ id: rid, comment }), m.requests_approved())}>
				{m.requests_approve()}
			</Button>
		{/if}
		{#if own && view}
			{@const rid = view.id}
			<Button size={40} scheme="g" variant="plain" disabled={busy} onclick={() => act(() => accessRequests.cancel({ id: rid }), m.requests_canceled())}>
				{m.common_cancel()}
			</Button>
		{/if}
	{/snippet}
</Drawer>

<style>
	.comment {
		margin-top: var(--miso-sp-16);
	}
</style>
