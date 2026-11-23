<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { terminalHref } from '#lib/terminal/socket.js';
	import type { AccessibleResource } from '#lib/gen/werbot/v1/resource_pb.js';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import type { Slot } from './dashboard';

	/** resources the caller may open in the browser terminal */
	let { accessible }: { accessible: Slot<AccessibleResource[]> } = $props();
</script>

<DashCard
	title={m.dashboard_terminals()}
	icon="desktop"
	href="/resources"
	slot={accessible}
	empty={{ icon: 'desktop', title: m.dashboard_no_terminals() }}
>
	{#snippet children(list)}
		<ul>
			{#each list as r (r.name)}
				<DashRow href={terminalHref(r.name)} title={r.name} icon="desktop">
					{#snippet meta()}<span>{m.terminal_open()}</span>{/snippet}
				</DashRow>
			{/each}
		</ul>
	{/snippet}
</DashCard>
