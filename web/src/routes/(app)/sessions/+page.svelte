<script lang="ts">
	import { page } from '$app/state';
	import { Badge, Button, Calendar, EmptyState, Label, Segmented, Select, Skeleton, Table, type SelectOption } from '#lib/ui/index.js';
	import { members, resources, sessions } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { bytes, elapsed, fmt } from '#lib/format.js';
	import Ago from '#lib/Ago.svelte';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import FilterMenu from '#lib/FilterMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import Pager from '#lib/Pager.svelte';
	import { canControlSession } from '#lib/sessions/access.js';
	import SessionMenu from '#lib/sessions/SessionMenu.svelte';
	import TerminateDrawer from '#lib/sessions/TerminateDrawer.svelte';
	import { dayStart, dayString } from '#lib/period.js';
	import { m } from '#lib/paraglide/messages.js';

	// members see only their own sessions and cannot list users or resources
	const manager = $derived(page.data.session?.role === Role.OWNER || page.data.session?.role === Role.ADMIN);

	let list = $state<Session[]>([]);
	let next = $state(0);
	let pageNo = $state(1);
	let failed = $state(false);
	// Retry bumps it to re-run the load effect
	let attempt = $state(0);
	let loaded = $state(false);
	let userId = $state('');
	let resourceId = $state('');
	let status = $state('0');
	let rangeStart = $state<Date | null>(null);
	let rangeEnd = $state<Date | null>(null);
	const from = $derived(dayString(rangeStart));
	const to = $derived(dayString(rangeEnd));
	let users = $state<SelectOption[]>([]);
	let resourceOptions = $state<SelectOption[]>([]);
	let menuOpen = $state('');
	let terminating = $state<string | null>(null);

	const statuses = $derived([
		{ value: '0', label: m.sessions_status_all() },
		{ value: '1', label: m.sessions_status_active() },
		{ value: '2', label: m.sessions_status_ended() }
	]);

	// reloads on every filter or page change; a superseded answer is dropped
	$effect(() => {
		void attempt;
		failed = false;
		let stale = false;
		const req = { userId, resourceId, status: Number(status), from: dayStart(from), to: dayStart(to, 1), page: pageNo };
		sessions.listSessions(req).then(
			(r) => {
				if (stale) return;
				({ sessions: list, nextPage: next } = r);
				failed = false;
				loaded = true;
			},
			(e) => {
				if (stale) return;
				failed = true;
				failToast(e);
			}
		);
		return () => (stale = true);
	});

	$effect(() => {
		if (!manager) return;
		Promise.all([members.listMembers({}), resources.listResources({})]).then(
			([mem, res]) => {
				users = mem.members.flatMap((x) => (x.user ? [{ value: x.user.id, label: x.user.login }] : []));
				resourceOptions = res.resources.map((r) => ({ value: r.id, label: r.name }));
			},
			failToast
		);
	});

	const activeFilters = $derived([userId, resourceId, from, to].filter(Boolean).length);

	const first = () => (pageNo = 1);

	function reset() {
		userId = resourceId = '';
		rangeStart = rangeEnd = null;
		first();
	}
</script>

{#snippet resetButton()}
	<Button scheme="g" variant="plain" size={28} iconLeft="close" onclick={reset}>{m.common_reset()}</Button>
{/snippet}

<PageHeader title={m.sessions_title()}>
	{#snippet toolbar()}
		<Segmented size={32} options={statuses} bind:value={status} onchange={first} />
		<FilterMenu active={activeFilters}>
			{#if manager}
				<Select label={m.sessions_filter_user()} options={[{ value: '', label: m.sessions_filter_user_all() }, ...users]} bind:value={userId} onchange={first} />
				<Select label={m.sessions_filter_resource()} options={[{ value: '', label: m.sessions_filter_resource_all() }, ...resourceOptions]} bind:value={resourceId} onchange={first} />
			{/if}
			{#snippet calendar()}
				<Calendar range bind:value={rangeStart} bind:endValue={rangeEnd} onselect={first} />
			{/snippet}
			{#snippet footer()}{#if activeFilters > 0}{@render resetButton()}{/if}{/snippet}
		</FilterMenu>
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={() => attempt++} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="history" title={m.sessions_empty()}>
			{#snippet actions()}{#if activeFilters > 0}{@render resetButton()}{/if}{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.sessions_col_resource()}</th>
				<th>{m.sessions_col_command()}</th>
				<th>{m.sessions_col_started()}</th>
				<th>{m.sessions_col_duration()}</th>
				<th>{m.sessions_col_traffic()}</th>
				<th></th>
			{/snippet}
			{#each list as s (s.id)}
				<tr>
					<td>
						<CellLead icon="person" avatar={s.userLogin} online={!s.endedAt}>
							<a class="row-name" href={`/sessions/${s.id}`}>{s.resourceName || s.id}</a>
							{#snippet sub()}
								<span>{s.userLogin || '—'}</span>
								{#if s.kind}<Label variant="soft" size={16}>{s.kind}</Label>{/if}
							{/snippet}
						</CellLead>
					</td>
					<td>
						{#if s.command}
							<div class="cmd">
								<code title={s.command}>{s.command.length > 80 ? s.command.slice(0, 80) + '…' : s.command}</code>
								<ClassChips classes={s.classes} />
							</div>
						{/if}
					</td>
					<td class="muted"><Ago ts={s.startedAt} /></td>
					<td>
						{#if s.endedAt}
							<span title={fmt(s.endedAt)}>{elapsed(s.startedAt, s.endedAt)}</span>
						{:else}
							<Badge tone="accent">{m.sessions_status_active()}</Badge>
						{/if}
					</td>
					<td class="muted">↑ {bytes(s.bytesUp)} ↓ {bytes(s.bytesDown)}</td>
					<td class="more">
						{#if s.live && canControlSession(page.data.session?.role, page.data.session?.user?.login, s.userLogin)}
							<SessionMenu id={s.id} onterminate={() => (terminating = s.id)} bind:open={() => menuOpen === s.id, (v) => (menuOpen = v ? s.id : '')} />
						{/if}
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
	<Pager bind:page={pageNo} {next} />
</PageBody>
<TerminateDrawer id={terminating} onclose={() => (terminating = null)} />

<style>
	.cmd {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
		align-items: flex-start;
	}

	code {
		font-family: var(--miso-font-mono);
		font-size: 12px;
		word-break: break-all;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}
</style>
