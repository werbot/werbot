<script lang="ts">
	import { Skeleton } from '#lib/ui/index.js';

	let { counters }: { counters: { loading: boolean; value: string; label: string }[] } = $props();
</script>

<div class="pills">
	{#each counters as c (c.label)}
		<div class="pills__item">
			{#if c.loading}<Skeleton variant="text" width={24} />{:else}<b class="pills__num">{c.value}</b>{/if}
			<span class="pills__label">{c.label}</span>
		</div>
	{/each}
</div>

<style>
	.pills {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(0, 1fr));
		grid-auto-flow: column;
		align-items: center;
		width: 100%;
		max-width: 928px;
		min-height: 64px;
		margin: 24px 0;
		padding: 8px;
		box-sizing: border-box;
		border-radius: 60px;
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
	}
	.pills__item {
		position: relative;
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 10px;
		padding: 0 12px;
	}
	.pills__item + .pills__item::before {
		content: '';
		position: absolute;
		left: -1px;
		top: 50%;
		transform: translateY(-50%);
		width: 1px;
		height: 28px;
		background: var(--miso-line-secondary);
	}
	.pills__num {
		font: var(--miso-font-ui-18-semi);
		color: var(--miso-txt-primary);
	}
	.pills__label {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
