<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Button, EmptyState, Skeleton, Table, Tabs, type TabOption } from '#lib/ui/index.js';
	import { accessRequests } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration } from '#lib/duration.js';
	import { fmt } from '#lib/format.js';
	import { RequestScope, type AccessRequest, type Grant } from '#lib/gen/werbot/v1/access_request_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import GrantsTable from '#lib/requests/GrantsTable.svelte';
	import NewRequestDrawer from '#lib/requests/NewRequestDrawer.svelte';
	import RequestDrawer from '#lib/requests/RequestDrawer.svelte';
	import StatusBadge from '#lib/requests/StatusBadge.svelte';
	import LabelChips from '#lib/resources/LabelChips.svelte';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	type Scope = 'mine' | 'to_approve' | 'all';
	const manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	const me = $derived(data.session.user?.id ?? '');

	let tab = $state('mine');
	let lists = $state<Record<Scope, AccessRequest[]>>({ mine: [], to_approve: [], all: [] });
	let grants = $state<Grant[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let creating = $state(false);

	// the email link /requests?id=<id> opens that request
	const selected = $derived(page.url.searchParams.get('id') ?? '');
	const show = (id: string) => goto(id ? `/requests?id=${id}` : '/requests', { reset: false });

	// ponytail: one page of the 200 newest per tab; add Pager when an organization outgrows it
	const list = (scope: RequestScope) => accessRequests.listRequests({ scope, pageSize: 200 }).then((r) => r.requests);

	async function refresh() {
		const [mine, toApprove, all, active] = await Promise.all([
			list(RequestScope.MINE),
			list(RequestScope.TO_APPROVE),
			manager ? list(RequestScope.ALL) : [],
			accessRequests.listGrants({ active: true })
		]);
		lists = { mine, to_approve: toApprove, all };
		grants = active.grants;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	const tabOptions = $derived<TabOption[]>([
		{ value: 'mine', label: m.requests_tab_mine() },
		{ value: 'to_approve', label: m.requests_tab_to_approve(), count: lists.to_approve.length },
		...(manager ? [{ value: 'all', label: m.requests_tab_all() }] : []),
		{ value: 'grants', label: m.requests_grants_title(), count: grants.length }
	]);
	const rows = $derived(tab === 'grants' ? [] : lists[tab as Scope]);

	onMount(load);
</script>

<PageHeader title={m.requests_title()}>
	{#snippet toolbar()}
		<Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.requests_new()}</Button>
	{/snippet}
	{#snippet tabs()}
		<Tabs options={tabOptions} bind:value={tab} />
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if tab === 'grants'}
		<GrantsTable {grants} {manager} {me} onchanged={load} />
	{:else if rows.length === 0}
		<EmptyState icon="front_hand" title={m.requests_empty()} />
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.requests_resource()}</th>
				<th>{m.requests_requester()}</th>
				<th>{m.requests_duration()}</th>
				<th>{m.requests_status()}</th>
				<th>{m.requests_expires()}</th>
				<th>{m.requests_created()}</th>
			{/snippet}
			{#each rows as r (r.id)}
				<tr>
					<td>
						<CellLead icon="front_hand">
							<button class="row-name" onclick={() => show(r.id)}>
								{#if r.resourceId}{r.resourceName}{:else}<LabelChips labels={r.labelSelector} />{/if}
							</button>
							{#snippet sub()}{r.reason}{/snippet}
						</CellLead>
					</td>
					<td>
						<CellLead icon="person" avatar={r.requesterLogin}>{r.requesterLogin}</CellLead>
					</td>
					<td class="muted">{r.kind === 'command' ? '—' : formatDuration(r.duration) || m.requests_duration_default()}</td>
					<td><StatusBadge status={r.status} /></td>
					<td class="muted">{r.status === 'pending' ? fmt(r.expiresAt) : '—'}</td>
					<td class="muted"><Ago ts={r.createdAt} /></td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<RequestDrawer id={selected} {me} onclose={() => show('')} onchanged={load} />
<NewRequestDrawer
	bind:open={creating}
	oncreated={(id) => {
		load();
		show(id);
	}}
/>

<style>
	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}
</style>
