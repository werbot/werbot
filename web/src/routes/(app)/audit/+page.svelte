<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, Calendar, EmptyState, Select, Label, Skeleton, Table, TextField, Tooltip, type SelectOption } from '#lib/ui/index.js';
	import { audit, members } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import EventDrawer from '#lib/audit/EventDrawer.svelte';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import FilterMenu from '#lib/FilterMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
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
	let failed = $state(false);
	// Retry bumps it to re-run the load effect
	let attempt = $state(0);
	let loaded = $state(false);
	let actors = $state<SelectOption[]>([]);
	let selected = $state('');

	const day = (v: string) => (v ? new Date(`${v}T00:00`) : null);

	const all = $derived({ value: '', label: m.audit_filter_actor_all() });

	// targets with a page of their own; a user target shows the login from the actor list
	const linked = ['resource', 'session'];
	const login = (ev: AuditEvent) => (ev.targetType === 'user' ? actors.find((a) => a.value === ev.targetId)?.label : undefined);

	$effect(() => {
		form = { action: query.action, actor: query.actor };
		rangeStart = day(query.from);
		rangeEnd = day(query.to);
		pageNo = 1;
	});

	$effect(() => {
		void attempt;
		failed = false;
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

	onMount(() => {
		members.listMembers({}).then(
			(r) => (actors = r.members.flatMap((x) => (x.user ? [{ value: x.user.id, label: x.user.login }] : []))),
			failToast
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
			{#snippet calendar()}
				<Calendar range bind:value={rangeStart} bind:endValue={rangeEnd} onselect={apply} />
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
		<EmptyState icon="report_detailed" title={m.audit_empty()}>
			{#snippet actions()}{#if activeFilters > 0}{@render resetButton()}{/if}{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.audit_col_action()}</th>
				<th>{m.audit_col_target()}</th>
				<th>{m.audit_col_time()}</th>
			{/snippet}
			{#each list as ev (ev.id)}
				<tr>
					<td>
						<!-- events without a user actor are the system's -->
						<CellLead icon="gear" avatar={ev.actorLogin || undefined}>
							<button class="row-name" onclick={() => (selected = ev.id)}>{ev.action}</button>
							{#snippet sub()}{ev.actorLogin || ev.actorKind}{/snippet}
						</CellLead>
					</td>
					<td>
						<span class="target">
							{#if ev.targetType}<Label variant="soft">{ev.targetType}</Label>{/if}
							{#if login(ev)}
								{login(ev)}
							{:else if ev.targetId}
								<Tooltip text={ev.targetId}>
									{#if linked.includes(ev.targetType)}
										<a class="id" href={`/${ev.targetType}s/${ev.targetId}`}>…{ev.targetId.slice(-8)}</a>
									{:else}
										<span class="id">…{ev.targetId.slice(-8)}</span>
									{/if}
								</Tooltip>
							{/if}
						</span>
					</td>
					<td class="muted"><Ago ts={ev.createdAt} /></td>
				</tr>
			{/each}
		</Table>
	{/if}
	<Pager bind:page={pageNo} {next} />
</PageBody>
<EventDrawer bind:id={selected} />

<style>
	.target {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.id {
		font-variant-numeric: tabular-nums;
		color: inherit;
	}

	a.id {
		color: var(--miso-txt-link);
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}
</style>
