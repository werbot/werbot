<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { onMount } from 'svelte';
	import { Button, EmptyState, InfoBlock, Label, MenuItem, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { mfa } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { EnrollTOTPResponse, Factor } from '#lib/gen/werbot/v1/mfa_pb.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PasskeyModal from '#lib/mfa/PasskeyModal.svelte';
	import RecoveryModal from '#lib/mfa/RecoveryModal.svelte';
	import TotpModal from '#lib/mfa/TotpModal.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { passkeysSupported } from '#lib/webauthn.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let factors = $state<Factor[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<Factor | null>(null);
	let passkeyOpen = $state(false);
	let enrollment = $state<EnrollTOTPResponse | null>(null);
	let codes = $state<string[]>([]);

	const hasTotp = $derived(factors.some((f) => f.kind === 'totp'));
	const hasRecovery = $derived(factors.some((f) => f.kind === 'recovery'));
	const kinds: Record<string, { label: () => string; icon: 'key' | 'phone' | 'lock' }> = {
		webauthn: { label: m.mfa_kind_passkey, icon: 'key' },
		totp: { label: m.mfa_kind_totp, icon: 'phone' },
		recovery: { label: m.mfa_kind_recovery, icon: 'lock' }
	};

	async function refresh() {
		factors = (await mfa.listFactors({})).factors;
		loaded = true;
		// the session's "enrollment required" flag follows the factors
		await invalidateAll();
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	async function startTotp() {
		try {
			enrollment = await mfa.enrollTOTP({});
		} catch (e) {
			failToast(e);
		}
	}

	async function generate() {
		try {
			codes = (await mfa.generateRecoveryCodes({})).codes;
			await load();
		} catch (e) {
			failToast(e);
		}
	}

	async function remove() {
		const factor = removing;
		removing = null;
		if (!factor) return;
		try {
			await mfa.deleteFactor({ id: factor.id });
			toast({ tone: 'success', text: m.common_deleted() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	function added() {
		toast({ tone: 'success', text: m.common_saved() });
		void load();
	}

	function confirmed() {
		enrollment = null;
		added();
	}

	onMount(load);
</script>

{#snippet adders()}
	{#if passkeysSupported()}
		<Button variant="solid" size={40} iconLeft="key" onclick={() => (passkeyOpen = true)}>{m.mfa_passkey_add()}</Button>
	{/if}
	{#if !hasTotp}
		<Button variant={passkeysSupported() ? 'plain' : 'solid'} size={40} iconLeft="phone" onclick={startTotp}>{m.mfa_totp_add()}</Button>
	{/if}
{/snippet}

<PageHeader title={m.mfa_title()}>
	{#snippet toolbar()}
		{#if factors.length > 0}
			{@render adders()}
			{#if !hasRecovery}
				<Button variant="plain" size={40} iconLeft="lock" onclick={generate}>{m.mfa_recovery_generate()}</Button>
			{/if}
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if data.session.mfaEnrollmentRequired}
		<InfoBlock tone="warning">{m.mfa_enrollment_required()}</InfoBlock>
	{/if}
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={3} />
	{:else if factors.length === 0}
		<EmptyState icon="lock" title={m.mfa_empty()}>
			{#snippet actions()}{@render adders()}{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.mfa_col_name()}</th>
				<th>{m.mfa_col_type()}</th>
				<th>{m.mfa_col_last_used()}</th>
				<th></th>
			{/snippet}
			{#each factors as f (f.id)}
				{@const kind = kinds[f.kind]}
				<tr>
					<td>
						<CellLead icon={kind?.icon ?? 'lock'}>
							{f.kind === 'webauthn' ? f.name : (kind?.label() ?? f.name)}
							{#snippet sub()}{f.kind === 'recovery' ? m.mfa_recovery_remaining({ count: f.remaining }) : ''}{/snippet}
						</CellLead>
					</td>
					<td><Label variant="soft">{kind?.label() ?? f.kind}</Label></td>
					<td class="muted">{#if f.lastUsedAt}<Ago ts={f.lastUsedAt} />{:else}{m.mfa_never_used()}{/if}</td>
					<td class="more">
						<RowMenu bind:open={() => menuOpen === f.id, (v) => (menuOpen = v ? f.id : '')}>
							{#snippet items(close)}
								{#if f.kind === 'recovery'}
									<MenuItem icon="repeat" onclick={() => (close(), generate())}>{m.mfa_recovery_regenerate()}</MenuItem>
								{/if}
								<MenuItem icon="trash" danger onclick={() => (close(), (removing = f))}>{m.mfa_delete()}</MenuItem>
							{/snippet}
						</RowMenu>
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<PasskeyModal bind:open={passkeyOpen} onadded={added} />
<TotpModal {enrollment} onclose={() => (enrollment = null)} onconfirmed={confirmed} />
<RecoveryModal {codes} onclose={() => (codes = [])} />

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.mfa_delete()}>
	<p>{m.mfa_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.mfa_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.more {
		text-align: right;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}
</style>
