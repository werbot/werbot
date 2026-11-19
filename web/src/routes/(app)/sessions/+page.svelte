<script lang="ts">
	import { page } from '$app/state';
	import { Avatar, Badge, Banner, Button, Calendar, EmptyState, Segmented, Select, Skeleton, Table, type SelectOption } from '#lib/ui/index.js';
	import { members, resources, sessions } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { bytes, fmt } from '#lib/format.js';
	import FilterMenu from '#lib/FilterMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import Pager from '#lib/Pager.svelte';
	import { dayStart, dayString } from '#lib/period.js';
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
	let rangeStart = $state<Date | null>(null);
	let rangeEnd = $state<Date | null>(null);
	const from = $derived(dayString(rangeStart));
	const to = $derived(dayString(rangeEnd));
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
		<Segmented size={28} options={statuses} bind:value={status} onchange={first} />
		<FilterMenu active={activeFilters}>
			{#if manager}
				<Select label={m.sessions_filter_user()} options={[{ value: '', label: m.sessions_filter_user_all() }, ...users]} bind:value={userId} onchange={first} />
				<Select label={m.sessions_filter_resource()} options={[{ value: '', label: m.sessions_filter_resource_all() }, ...resourceOptions]} bind:value={resourceId} onchange={first} />
			{/if}
			<Calendar range bind:value={rangeStart} bind:endValue={rangeEnd} onselect={first} />
			{#if activeFilters > 0}{@render resetButton()}{/if}
		</FilterMenu>
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if error || optionsError}<Banner tone="alert">{error || optionsError}</Banner>{/if}

	{#if error}
		<!-- the error banner above says why -->
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="history" title={m.sessions_empty()}>
			{#snippet actions()}{#if activeFilters > 0}{@render resetButton()}{/if}{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.sessions_col_user()}</th>
				<th>{m.sessions_col_resource()}</th>
				<th>{m.sessions_col_started()}</th>
				<th>{m.sessions_col_ended()}</th>
				<th>{m.sessions_col_status()}</th>
				<th>{m.sessions_col_traffic()}</th>
			{/snippet}
			{#each list as s (s.id)}
				<tr>
					<td>
						{#if s.userLogin}<span class="user"><Avatar name={s.userLogin} size={24} />{s.userLogin}</span>{:else}—{/if}
					</td>
					<td><a class="name" href={`/sessions/${s.id}`}>{s.resourceName || s.id}</a></td>
					<td>{fmt(s.startedAt)}</td>
					<td>{fmt(s.endedAt)}</td>
					<td>
						<Badge tone={s.endedAt ? 'ghost' : 'accent'}>
							{s.endedAt ? m.sessions_status_ended() : m.sessions_status_active()}
						</Badge>
					</td>
					<td>↑ {bytes(s.bytesUp)} ↓ {bytes(s.bytesDown)}</td>
				</tr>
			{/each}
		</Table>
	{/if}
	<Pager bind:page={pageNo} {next} />
</PageBody>

<style>
	.name {
		font-weight: 600;
		color: inherit;
		text-decoration: none;
	}

	.name:focus-visible {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}

	.user {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
	}
</style>
