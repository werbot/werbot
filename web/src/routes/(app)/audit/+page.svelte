<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Avatar, Banner, Button, Calendar, EmptyState, Select, Skeleton, Table, TextField, type SelectOption } from '#lib/ui/index.js';
	import { audit, members } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import EventDrawer from '#lib/audit/EventDrawer.svelte';
	import { fmt } from '#lib/format.js';
	import FilterMenu from '#lib/FilterMenu.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import Pager from '#lib/Pager.svelte';
	import { dayStart, dayString } from '#lib/period.js';
	import { m } from '#lib/paraglide/messages.js';

	const keys = ['action', 'actor', 'from', 'to'] as const;
	type Filter = Record<(typeof keys)[number], string>;

	// the query string holds the filter, so a filtered view can be linked
	const query = $derived(Object.fromEntries(keys.map((k) => [k, page.url.searchParams.get(k) ?? ''])) as Filter);

	let form = $state({ action: '', actor: '' });
	let rangeStart = $state<Date | null>(null);
	let rangeEnd = $state<Date | null>(null);
	let list = $state<AuditEvent[]>([]);
	let next = $state(0);
	let pageNo = $state(1);
	let error = $state('');
	let loaded = $state(false);
	let actors = $state<SelectOption[]>([]);
	let selected = $state('');

	const day = (v: string) => (v ? new Date(`${v}T00:00`) : null);

	const all = $derived({ value: '', label: m.audit_filter_actor_all() });

	$effect(() => {
		form = { action: query.action, actor: query.actor };
		rangeStart = day(query.from);
		rangeEnd = day(query.to);
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
		const q = new URLSearchParams(Object.entries({ ...form, from: dayString(rangeStart), to: dayString(rangeEnd) }).filter(([, v]) => v));
		goto(`?${q}`, { replace: true, reset: false });
	}

	const activeFilters = $derived(keys.filter((k) => query[k]).length);

	function reset() {
		form = { action: '', actor: '' };
		rangeStart = rangeEnd = null;
		apply();
	}
</script>

{#snippet resetButton()}
	<Button scheme="g" variant="plain" size={28} iconLeft="close" onclick={reset}>{m.common_reset()}</Button>
{/snippet}

<PageHeader title={m.audit_title()}>
	{#snippet toolbar()}
		<FilterMenu active={activeFilters}>
			<TextField label={m.audit_filter_action()} placeholder={m.audit_filter_action_hint()} maxlength={64} bind:value={form.action} onchange={apply} />
			<Select label={m.audit_filter_actor()} options={[all, ...actors]} bind:value={form.actor} onchange={apply} />
			<Calendar range bind:value={rangeStart} bind:endValue={rangeEnd} onselect={apply} />
			{#if activeFilters > 0}{@render resetButton()}{/if}
		</FilterMenu>
	{/snippet}
</PageHeader>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

{#if error}
	<!-- the error banner above says why -->
{:else if !loaded}
	<Skeleton variant="text" lines={6} />
{:else if list.length === 0}
	<EmptyState icon="history" title={m.audit_empty()}>
		{#snippet actions()}{#if activeFilters > 0}{@render resetButton()}{/if}{/snippet}
	</EmptyState>
{:else}
	<Table>
		{#snippet header()}
			<th>{m.audit_col_time()}</th>
			<th>{m.audit_col_actor()}</th>
			<th>{m.audit_col_action()}</th>
			<th>{m.audit_col_target()}</th>
		{/snippet}
		{#each list as ev (ev.id)}
			<tr>
				<td>{fmt(ev.createdAt)}</td>
				<td>
					{#if ev.actorLogin}<span class="actor"><Avatar name={ev.actorLogin} size={24} />{ev.actorLogin}</span>{:else}{ev.actorKind}{/if}
				</td>
				<td><button class="action" onclick={() => (selected = ev.id)}>{ev.action}</button></td>
				<td>{ev.targetType} {ev.targetId}</td>
			</tr>
		{/each}
	</Table>
{/if}
<Pager bind:page={pageNo} {next} />
<EventDrawer bind:id={selected} />

<style>
	.actor {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
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
