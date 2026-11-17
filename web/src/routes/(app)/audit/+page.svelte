<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, EmptyState, Select, Table, TextField, type SelectOption } from '#lib/ui/index.js';
	import { audit, members } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import EventDrawer from '#lib/audit/EventDrawer.svelte';
	import { fmt } from '#lib/format.js';
	import Pager from '#lib/Pager.svelte';
	import { dayStart } from '#lib/period.js';
	import { m } from '#lib/paraglide/messages.js';

	const keys = ['action', 'actor', 'from', 'to'] as const;
	type Filter = Record<(typeof keys)[number], string>;

	// the query string holds the filter, so a filtered view can be linked
	const query = $derived(Object.fromEntries(keys.map((k) => [k, page.url.searchParams.get(k) ?? ''])) as Filter);

	let form = $state<Filter>({ action: '', actor: '', from: '', to: '' });
	let list = $state<AuditEvent[]>([]);
	let next = $state(0);
	let pageNo = $state(1);
	let error = $state('');
	let loaded = $state(false);
	let actors = $state<SelectOption[]>([]);
	let selected = $state('');

	const all = $derived({ value: '', label: m.audit_filter_actor_all() });

	$effect(() => {
		form = { ...query };
		pageNo = 1;
	});

	$effect(() => {
		let stale = false;
		const req = {
			action: query.action,
			actorUserId: query.actor,
			from: dayStart(query.from),
			to: dayStart(query.to, 1),
			page: pageNo
		};
		audit.listEvents(req).then(
			(r) => {
				if (stale) return;
				({ events: list, nextPage: next } = r);
				error = '';
				loaded = true;
			},
			(e) => {
				if (!stale) error = errorMessage(e);
			}
		);
		return () => (stale = true);
	});

	onMount(() => {
		members.listMembers({}).then(
			(r) => (actors = r.members.flatMap((x) => (x.user ? [{ value: x.user.id, label: x.user.login }] : []))),
			(e) => (error = errorMessage(e))
		);
	});

	function apply() {
		const q = new URLSearchParams(Object.entries(form).filter(([, v]) => v));
		goto(`?${q}`, { replace: true, reset: false });
	}
</script>

<h1>{m.audit_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

<form
	class="filters"
	onsubmit={(e) => {
		e.preventDefault();
		apply();
	}}
>
	<TextField label={m.audit_filter_action()} placeholder={m.audit_filter_action_hint()} maxlength={64} bind:value={form.action} onchange={apply} />
	<Select label={m.audit_filter_actor()} options={[all, ...actors]} bind:value={form.actor} onchange={apply} />
	<TextField type="date" label={m.audit_filter_from()} bind:value={form.from} onchange={apply} />
	<TextField type="date" label={m.audit_filter_to()} bind:value={form.to} onchange={apply} />
</form>

{#if !loaded || error}
	<!-- nothing to show yet, or the error banner above says why -->
{:else if list.length === 0}
	<EmptyState title={m.audit_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.audit_col_time()}</th>
			<th>{m.audit_col_actor()}</th>
			<th>{m.audit_col_action()}</th>
			<th>{m.audit_col_target()}</th>
		{/snippet}
		{#each list as ev (ev.id)}
			<tr onclick={() => (selected = ev.id)}>
				<td>{fmt(ev.createdAt)}</td>
				<td>{ev.actorLogin || ev.actorKind}</td>
				<td><button class="action">{ev.action}</button></td>
				<td>{ev.targetType} {ev.targetId}</td>
			</tr>
		{/each}
	</Table>
{/if}
<Pager bind:page={pageNo} {next} />
<EventDrawer bind:id={selected} />

<style>
	.filters {
		display: flex;
		flex-wrap: wrap;
		align-items: flex-end;
		gap: 12px;
		margin-bottom: 16px;
	}

	tr {
		cursor: pointer;
	}

	.action {
		padding: 0;
		font: inherit;
		color: inherit;
		background: none;
		border: 0;
		cursor: pointer;
	}
</style>
