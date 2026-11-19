<script lang="ts">
	import { Badge, Label, ProgressBar } from '#lib/ui/index.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import { onlineCount, type Slot } from './dashboard';

	let { pools }: { pools: Slot<Gateway[]> } = $props();
</script>

<DashCard
	title={m.dashboard_pools()}
	icon="shield"
	href="/gateways"
	slot={pools}
	count={pools.state === 'ok' && pools.value.length ? String(pools.value.length) : undefined}
	empty={{ icon: 'shield', title: m.gateways_empty() }}
>
	{#snippet children(list)}
		<ul>
			{#each list as g (g.id)}
				{@const inst = onlineCount(g.instances)}
				<DashRow href="/gateways" title={g.name} icon="hub">
					{#snippet meta()}
						{#each g.connectors as c (c)}<Label variant="soft" size={16}>{c}</Label>{:else}—{/each}
					{/snippet}
					{#snippet trail()}
						<Badge tone={g.online ? 'accent' : 'ghost'}>{g.online ? m.gateways_online() : m.gateways_offline()}</Badge>
						<span class="inst" title={m.gateways_instances_title()}>
							<span class="inst__bar"><ProgressBar value={inst.total ? inst.online / inst.total : 0} thickness={4} /></span>
							{inst.online}/{inst.total}
						</span>
					{/snippet}
				</DashRow>
			{/each}
		</ul>
	{/snippet}
</DashCard>

<style>
	.inst {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-06);
	}
	.inst__bar {
		width: 48px;
	}
</style>
