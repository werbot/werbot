<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Button, EmptyState, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { agents } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration } from '#lib/duration.js';
	import type { Agent, AgentLimits, AgentRun } from '#lib/gen/werbot/v1/agent_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import AgentMenu from '#lib/agents/AgentMenu.svelte';
	import KillSwitchDrawer, { type KillTarget } from '#lib/agents/KillSwitchDrawer.svelte';
	import LimitsDrawer from '#lib/agents/LimitsDrawer.svelte';
	import ProfilesDrawer from '#lib/agents/ProfilesDrawer.svelte';
	import RunsTable from '#lib/agents/RunsTable.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);

	let list = $state<Agent[]>([]);
	let runs = $state<AgentRun[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let limiting = $state<Agent | null>(null);
	let profiling = $state<Agent | null>(null);
	let removing = $state<Agent | null>(null);
	let kill = $state<KillTarget | null>(null);

	async function refresh() {
		const [a, r] = await Promise.all([agents.listAgents({}), agents.listRuns({ pageSize: 20 })]);
		list = a.agents;
		runs = r.runs;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	// 0 or unset is the server default
	const limitsText = (l?: AgentLimits) =>
		[l?.maxActions || m.agents_limits_default(), formatDuration(l?.maxDuration) || m.agents_limits_default(), l?.maxConcurrent || m.agents_limits_default()].join(' / ');

	async function act(call: () => Promise<unknown>, message: string) {
		try {
			await call();
			toast({ tone: 'success', text: message });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	const freeze = (a: Agent) =>
		act(() => agents.updateAgent({ id: a.id, status: a.status === 'frozen' ? 'active' : 'frozen' }), m.common_saved());

	async function remove() {
		const a = removing;
		removing = null;
		if (a) await act(() => agents.deleteAgent({ id: a.id }), m.common_deleted());
	}

	const killAgent = (a: Agent, type: 'agent' | 'agents_of_user') =>
		(kill = type === 'agent' ? { type, id: a.id, title: `${m.agents_kill_agent()}: ${a.name}` } : { type, id: a.sponsorUserId, title: `${m.agents_kill_sponsor()}: ${a.sponsorLogin}` });

	onMount(() => {
		load();
		// silent refresh: a failed poll keeps the last list
		const timer = setInterval(() => refresh().catch(() => {}), 10_000);
		return () => clearInterval(timer);
	});
</script>

<PageHeader title={m.agents_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if manager}
			<Button scheme="w" variant="solid" size={40} iconLeft="lock" onclick={() => (kill = { type: 'all_agents', id: '', title: m.agents_kill_all() })}>{m.agents_kill_all()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="activity" title={m.agents_empty()} />
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.agents_col_name()}</th>
				{#if manager}<th>{m.agents_col_sponsor()}</th>{/if}
				<th>{m.agents_col_status()}</th>
				<th>{m.agents_col_limits()}</th>
				<th>{m.agents_col_active_runs()}</th>
				<th></th>
			{/snippet}
			{#each list as a (a.id)}
				<tr>
					<td><CellLead icon="activity"><span class="name">{a.name}</span></CellLead></td>
					{#if manager}<td class="muted">{a.sponsorLogin}</td>{/if}
					<td><Badge tone={a.status === 'frozen' ? 'upcoming' : 'accent'}>{a.status === 'frozen' ? m.agents_status_frozen() : m.agents_status_active()}</Badge></td>
					<td class="muted">{limitsText(a.limits)}</td>
					<td class="muted">{a.activeRuns}</td>
					<td class="more">
						<AgentMenu
							agent={a}
							{manager}
							bind:open={() => menuOpen === a.id, (v) => (menuOpen = v ? a.id : '')}
							onlimits={() => (limiting = a)}
							onprofiles={() => (profiling = a)}
							onfreeze={() => freeze(a)}
							onkill={(type) => killAgent(a, type)}
							ondelete={() => (removing = a)}
						/>
					</td>
				</tr>
			{/each}
		</Table>
		<h2>{m.agents_runs_title()}</h2>
		{#if runs.length === 0}
			<p class="muted empty">{m.agents_runs_empty()}</p>
		{:else}
			<RunsTable {runs} />
		{/if}
	{/if}
</PageBody>

<LimitsDrawer agent={limiting} onclose={() => (limiting = null)} onsaved={load} />
<ProfilesDrawer agent={profiling} onclose={() => (profiling = null)} />
<KillSwitchDrawer target={kill} onclose={() => (kill = null)} />

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.agents_action_delete()}>
	<p>{m.agents_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.agents_action_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.name {
		font-weight: 600;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}

	h2 {
		margin: var(--miso-sp-24) var(--miso-sp-16) var(--miso-sp-08);
		font: var(--miso-font-ui-14-semi);
	}

	.empty {
		margin: var(--miso-sp-08) var(--miso-sp-16);
	}
</style>
