<script lang="ts">
	import { Table } from '#lib/ui/index.js';
	import type { AgentRun } from '#lib/gen/werbot/v1/agent_pb.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import RunStatus from './RunStatus.svelte';

	let { runs }: { runs: AgentRun[] } = $props();
</script>

<Table>
	{#snippet header()}
		<th>{m.agents_col_name()}</th>
		<th>{m.agents_run_client()}</th>
		<th>{m.agents_col_status()}</th>
		<th>{m.agents_run_actions()}</th>
		<th>{m.agents_run_started()}</th>
	{/snippet}
	{#each runs as r (r.id)}
		<tr>
			<td>
				<CellLead icon="activity">
					<a class="row-name" href={`/agents/runs/${r.id}`}>{r.agentName}</a>
					{#snippet sub()}{r.reason}{/snippet}
				</CellLead>
			</td>
			<td class="muted">{r.client || '—'}</td>
			<td><RunStatus status={r.status} /></td>
			<td class="muted">{r.actions} / {r.maxActions}</td>
			<td class="muted"><Ago ts={r.startedAt} /></td>
		</tr>
	{/each}
</Table>

<style>
	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}
</style>
