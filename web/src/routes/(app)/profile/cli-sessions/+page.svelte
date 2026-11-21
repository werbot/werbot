<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampMs } from '@bufbuild/protobuf/wkt';
	import { Button, EmptyState, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { cliSessions } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { fmt } from '#lib/format.js';
	import type { CLISession } from '#lib/gen/werbot/v1/cli_session_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<CLISession[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let revoking = $state<CLISession | null>(null);
	let menuOpen = $state('');

	async function load() {
		try {
			({ sessions: list } = await cliSessions.listCLISessions({}));
			loaded = true;
			failed = false;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	async function revoke() {
		const s = revoking;
		revoking = null;
		if (!s) return;
		try {
			await cliSessions.revokeCLISession({ id: s.id });
			toast({ tone: 'success', text: m.common_deleted() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	// a session ends at the idle deadline or the absolute one, whichever comes first
	const ends = (s: CLISession) =>
		s.expiresAt && s.absoluteExpiresAt && timestampMs(s.absoluteExpiresAt) < timestampMs(s.expiresAt) ? s.absoluteExpiresAt : s.expiresAt;

	onMount(load);
</script>

<PageHeader title={m.cli_sessions_title()} count={loaded ? list.length : undefined} />
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else if list.length === 0}
		<EmptyState icon="laptop" title={m.cli_sessions_empty()} />
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.cli_sessions_col_host()}</th>
				<th>{m.cli_sessions_col_ip()}</th>
				<th>{m.cli_sessions_col_created()}</th>
				<th>{m.cli_sessions_col_last_used()}</th>
				<th>{m.cli_sessions_col_expires()}</th>
				<th></th>
			{/snippet}
			{#each list as s (s.id)}
				<tr>
					<td>
						<CellLead icon="laptop">
							<span class="host">{s.hostname || '—'}</span>
							{#snippet sub()}{s.clientId}{/snippet}
						</CellLead>
					</td>
					<td class="muted">{s.ip || '—'}</td>
					<td class="muted"><Ago ts={s.createdAt} /></td>
					<td class="muted"><Ago ts={s.lastUsedAt} /></td>
					<td class="muted">{fmt(ends(s))}</td>
					<td class="more">
						<RowMenu label={m.cli_sessions_revoke()} onselect={() => (revoking = s)} bind:open={() => menuOpen === s.id, (v) => (menuOpen = v ? s.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<Modal open={revoking !== null} onclose={() => (revoking = null)} title={m.cli_sessions_revoke()}>
	<p>{m.cli_sessions_revoke_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (revoking = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={revoke}>{m.cli_sessions_revoke()}</Button>
	{/snippet}
</Modal>

<style>
	.host {
		font-weight: 600;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}
</style>
