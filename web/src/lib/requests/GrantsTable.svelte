<script lang="ts">
	import { Badge, Button, EmptyState, Modal, Table, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import { accessRequests } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { fmt } from '#lib/format.js';
	import type { Grant } from '#lib/gen/werbot/v1/access_request_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** active grants: the caller's own, or every one for owner and admin */
		grants: Grant[];
		manager: boolean;
		me: string;
		onchanged: () => void;
	}

	let { grants, manager, me, onchanged }: Props = $props();

	let revoking = $state<Grant | null>(null);
	let reason = $state('');
	let menuOpen = $state('');

	async function run(call: () => Promise<unknown>, message: string) {
		try {
			await call();
			toast({ tone: 'success', text: message });
		} catch (e) {
			failToast(e);
		}
		onchanged();
	}

	function revoke() {
		const grant = revoking;
		revoking = null;
		if (grant) run(() => accessRequests.revokeGrant({ id: grant.id, reason }), m.requests_grant_revoked());
		reason = '';
	}
</script>

{#if grants.length === 0}
	<EmptyState icon="key" title={m.requests_grants_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.requests_resource()}</th>
			{#if manager}<th>{m.requests_grant_user()}</th>{/if}
			<th>{m.requests_expires()}</th>
			<th></th>
		{/snippet}
		{#each grants as grant (grant.id)}
			<tr>
				<td>
					<CellLead icon="key">
						<span class="name">{grant.resourceName}</span>
						{#snippet sub()}
							{#if grant.breakGlass}<Badge tone="alert">{m.requests_break_glass()}</Badge>{/if}
							{#if grant.needsReview}<Badge tone="upcoming">{m.requests_needs_review()}</Badge>{/if}
						{/snippet}
					</CellLead>
				</td>
				{#if manager}<td>{grant.userLogin}</td>{/if}
				<td class="muted">{fmt(grant.expiresAt)}</td>
				<td class="more">
					{#if manager && grant.needsReview && grant.userId !== me}
						<Button variant="plain" onclick={() => run(() => accessRequests.reviewBreakGlass({ grantId: grant.id }), m.requests_grant_reviewed())}>
							{m.requests_grant_review()}
						</Button>
					{/if}
					<RowMenu label={m.requests_grant_revoke()} onselect={() => (revoking = grant)} bind:open={() => menuOpen === grant.id, (v) => (menuOpen = v ? grant.id : '')} />
				</td>
			</tr>
		{/each}
	</Table>
{/if}

<Modal open={revoking !== null} onclose={() => (revoking = null)} title={m.requests_grant_revoke()} width={480}>
	<form id="revoke-grant" onsubmit={(e) => (e.preventDefault(), revoke())}>
		<p>{m.requests_grant_revoke_confirm()}</p>
		<FormField label={m.requests_reason()} bind:value={reason} maxlength={1024} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (revoking = null)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="revoke-grant" scheme="w" variant="solid">{m.requests_grant_revoke()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	form p {
		margin: 0;
	}

	.name {
		font-weight: 600;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
		white-space: nowrap;
	}
</style>
