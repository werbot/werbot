<script lang="ts">
	import { Icon } from '#lib/ui/index.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import HealthDot from '#lib/resources/HealthDot.svelte';
	import { typeIcon } from '#lib/resources/icons.js';
	import Ago from '#lib/Ago.svelte';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import { unhealthy, type Slot } from './dashboard';

	let { resources }: { resources: Slot<Resource[]> } = $props();

	const failing = $derived(resources.state === 'ok' ? unhealthy(resources.value) : []);
</script>

<DashCard
	title={m.dashboard_resource_health()}
	icon="activity"
	href="/resources"
	slot={resources}
	count={failing.length ? String(failing.length) : undefined}
	empty={{ icon: 'desktop', title: m.resources_empty() }}
>
	{#snippet children(list)}
		{#if failing.length}
			<ul>
				{#each failing as r (r.id)}
					<DashRow href={`/resources/${r.id}`} title={r.name} icon={typeIcon(r.type)}>
						{#snippet dot()}<HealthDot health={r.health} />{/snippet}
						{#snippet meta()}<span title={r.health?.error}>{r.health?.error}</span>{/snippet}
						{#snippet trail()}<Ago ts={r.health?.checkedAt} />{/snippet}
					</DashRow>
				{/each}
			</ul>
		{:else}
			<div class="ok">
				<span class="ok__icon"><Icon name="check_circle" size={32} /></span>
				<b class="ok__title">{m.dashboard_no_health_errors({ count: list.length })}</b>
			</div>
		{/if}
	{/snippet}
</DashCard>

<style>
	/* same layout as the miso-ui EmptyState, with a check icon */
	.ok {
		display: flex;
		flex: 1;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--miso-sp-08);
		padding: 32px 16px;
		text-align: center;
	}
	.ok__icon {
		display: inline-flex;
		color: var(--miso-green-500);
	}
	.ok__title {
		font: var(--miso-font-ui-14-semi);
		color: var(--miso-txt-secondary);
	}
</style>
