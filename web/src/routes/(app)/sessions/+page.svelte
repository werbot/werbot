<script lang="ts">
	import { page } from '$app/state';
	import { Badge, Banner, EmptyState, Select, Table, TextField, type SelectOption } from '#lib/ui/index.js';
	import { members, resources, sessions } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { fmt } from '#lib/format.js';
	import Pager from '#lib/Pager.svelte';
	import { dayStart } from '#lib/period.js';
	import { m } from '#lib/paraglide/messages.js';

	// members see only their own sessions and cannot list users or resources
	const manager = $derived(page.data.session?.role === Role.OWNER || page.data.session?.role === Role.ADMIN);

	let list = $state<Session[]>([]);
	let next = $state(0);
	let pageNo = $state(1);
	let error = $state('');
	let loaded = $state(false);
	let optionsError = $state('');
	let userId = $state('');
	let resourceId = $state('');
	let status = $state('0');
	let from = $state('');
	let to = $state('');
	let users = $state<SelectOption[]>([]);
	let resourceOptions = $state<SelectOption[]>([]);

	const statuses = $derived([
		{ value: '0', label: m.sessions_status_all() },
		{ value: '1', label: m.sessions_status_active() },
		{ value: '2', label: m.sessions_status_ended() }
	]);

	// reloads on every filter or page change; a superseded answer is dropped
	$effect(() => {
		let stale = false;
		const req = { userId, resourceId, status: Number(status), from: dayStart(from), to: dayStart(to, 1), page: pageNo };
		sessions.listSessions(req).then(
			(r) => {
				if (stale) return;
				({ sessions: list, nextPage: next } = r);
				error = '';
				loaded = true;
			},
			(e) => {
				if (!stale) error = errorMessage(e);
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
			(e) => (optionsError = errorMessage(e))
		);
	});

	const first = () => (pageNo = 1);
</script>

<h1>{m.sessions_title()}</h1>
{#if error || optionsError}<Banner tone="alert">{error || optionsError}</Banner>{/if}

<div class="filters">
	{#if manager}
		<Select label={m.sessions_filter_user()} options={[{ value: '', label: m.sessions_filter_user_all() }, ...users]} bind:value={userId} onchange={first} />
		<Select label={m.sessions_filter_resource()} options={[{ value: '', label: m.sessions_filter_resource_all() }, ...resourceOptions]} bind:value={resourceId} onchange={first} />
	{/if}
	<Select label={m.sessions_filter_status()} options={statuses} bind:value={status} onchange={first} />
	<TextField type="date" label={m.sessions_filter_from()} bind:value={from} onchange={first} />
	<TextField type="date" label={m.sessions_filter_to()} bind:value={to} onchange={first} />
</div>

{#if !loaded || error}
	<!-- nothing to show yet, or the error banner above says why -->
{:else if list.length === 0}
	<EmptyState title={m.sessions_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.sessions_col_user()}</th>
			<th>{m.sessions_col_resource()}</th>
			<th>{m.sessions_col_started()}</th>
			<th>{m.sessions_col_ended()}</th>
			<th>{m.sessions_col_status()}</th>
		{/snippet}
		{#each list as s (s.id)}
			<tr>
				<td>{s.userLogin || '—'}</td>
				<td><a href={`/sessions/${s.id}`}>{s.resourceName || s.id}</a></td>
				<td>{fmt(s.startedAt)}</td>
				<td>{fmt(s.endedAt)}</td>
				<td>
					<Badge tone={s.endedAt ? 'ghost' : 'accent'}>
						{s.endedAt ? m.sessions_status_ended() : m.sessions_status_active()}
					</Badge>
				</td>
			</tr>
		{/each}
	</Table>
{/if}
<Pager bind:page={pageNo} {next} />

<style>
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 12px;
		margin-bottom: 16px;
	}
</style>
