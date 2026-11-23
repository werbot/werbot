<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Button, EmptyState, PropertyRow, Skeleton, toast } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import { agents } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { fmt } from '#lib/format.js';
	import type { GetRunResponse } from '#lib/gen/werbot/v1/agent_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import KillSwitchDrawer, { type KillTarget } from '#lib/agents/KillSwitchDrawer.svelte';
	import RunStatus from '#lib/agents/RunStatus.svelte';
	import SessionDrawer from '#lib/agents/SessionDrawer.svelte';
	import TimelineList from '#lib/agents/TimelineList.svelte';
	import { buildTimeline, filterTimeline, type TimelineFilter } from '#lib/agents/timeline.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	const id = $derived(page.params.id ?? '');

	let detail = $state<GetRunResponse>();
	let sponsor = $state('');
	let missing = $state(false);
	let failed = $state(false);
	let filter = $state('all');
	let sessionId = $state('');
	let kill = $state<KillTarget | null>(null);

	const run = $derived(detail?.run);
	const live = $derived(run?.status === 'running' || run?.status === 'paused');
	const items = $derived(detail ? filterTimeline(buildTimeline(detail), filter as TimelineFilter) : []);
	const filters = $derived([
		{ value: 'all', label: m.agents_timeline_filter_all() },
		{ value: 'denials', label: m.agents_timeline_filter_denials() },
		{ value: 'approvals', label: m.agents_timeline_filter_approvals() },
		{ value: 'dangerous', label: m.agents_timeline_filter_dangerous() }
	]);

	async function refresh() {
		detail = await agents.getRun({ id });
		// the run names its agent only; the sponsor's login comes from the agent list, once
		if (!sponsor && detail.run) sponsor = (await agents.listAgents({})).agents.find((a) => a.id === detail?.run?.agentId)?.sponsorLogin ?? '';
	}

	async function load() {
		try {
			await refresh();
			failed = missing = false;
		} catch (e) {
			missing = ConnectError.from(e).code === Code.NotFound;
			failed = !missing;
			if (failed) failToast(e);
		}
	}

	async function act(call: () => Promise<unknown>, message: string) {
		try {
			await call();
			toast({ tone: 'success', text: message });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(() => {
		load();
		// silent refresh while the run is live: a failed poll keeps the last state
		const timer = setInterval(() => live && refresh().catch(() => {}), 5000);
		return () => clearInterval(timer);
	});
</script>

<PageHeader title={run?.agentName ?? m.agents_run_title()} breadcrumbs={[{ label: m.agents_title(), href: '/agents' }, { label: run?.agentName ?? m.agents_run_title() }]} />
<PageBody>
	{#if missing}
		<EmptyState icon="cloud" title={m.agents_run_not_found()} />
	{:else if failed}
		<LoadFailed onretry={load} />
	{:else if !detail || !run}
		<Skeleton variant="rect" height={240} />
	{:else}
		<div class="detail">
			<div class="detail__main">
				<div class="filter"><FilledSelect label={m.common_filter()} options={filters} bind:value={filter} /></div>
				<TimelineList {items} onsession={(sid) => (sessionId = sid)} />
			</div>
			<aside class="detail__props">
				<PropertyRow label={m.agents_run_agent()}>{run.agentName}</PropertyRow>
				<PropertyRow label={m.agents_run_sponsor()}>{sponsor || '—'}</PropertyRow>
				<PropertyRow label={m.agents_run_client()}>{run.client || '—'}</PropertyRow>
				<PropertyRow label={m.agents_run_reason()}>{run.reason || '—'}</PropertyRow>
				<PropertyRow label={m.agents_col_status()}><RunStatus status={run.status} /></PropertyRow>
				<PropertyRow label={m.agents_run_actions()}>{run.actions} / {run.maxActions}</PropertyRow>
				<PropertyRow label={m.agents_run_denials()}>{run.denialsInRow}</PropertyRow>
				<PropertyRow label={m.agents_run_started()}>{fmt(run.startedAt)}</PropertyRow>
				<PropertyRow label={m.agents_run_expires()}>{fmt(run.expiresAt)}</PropertyRow>
				{#if run.endedAt}<PropertyRow label={m.agents_run_ended()}>{fmt(run.endedAt)}</PropertyRow>{/if}
				{#if run.endReason}<PropertyRow label={m.agents_run_end_reason()}>{run.endReason}</PropertyRow>{/if}
				<div class="actions">
					{#if run.status === 'paused'}
						<Button size={40} variant="solid" onclick={() => act(() => agents.resumeRun({ id }), m.common_saved())}>{m.agents_run_resume()}</Button>
					{/if}
					{#if live}
						<Button size={40} scheme="g" variant="plain" onclick={() => act(() => agents.endRun({ id }), m.common_saved())}>{m.agents_run_end()}</Button>
					{/if}
					{#if manager}
						<Button size={40} scheme="w" variant="solid" iconLeft="lock" onclick={() => (kill = { type: 'agent_run', id, title: m.agents_run_lock() })}>{m.agents_run_lock()}</Button>
					{/if}
				</div>
			</aside>
		</div>
	{/if}
</PageBody>

<SessionDrawer session={detail?.sessions.find((s) => s.id === sessionId) ?? null} onclose={() => (sessionId = '')} />
<KillSwitchDrawer target={kill} onclose={() => (kill = null)} />

<style>
	.detail {
		display: grid;
		grid-template-columns: 1fr 320px;
		gap: var(--miso-sp-16);
		align-items: start;
	}

	.detail__main {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-16);
		min-width: 0;
	}

	.filter {
		max-width: 240px;
	}

	.detail__props {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-08);
		padding: var(--miso-sp-16);
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08);
		margin-top: var(--miso-sp-16);
	}
</style>
