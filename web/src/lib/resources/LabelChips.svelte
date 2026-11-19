<script lang="ts">
	import { Label, Tooltip } from '#lib/ui/index.js';

	let { labels, max = 3 }: { labels: Record<string, string>; max?: number } = $props();

	const pairs = $derived(Object.entries(labels).map(([k, v]) => `${k}=${v}`));
</script>

<span class="chips">
	{#each pairs.slice(0, max) as pair (pair)}<Label variant="soft">{pair}</Label>{/each}
	{#if pairs.length > max}<Tooltip text={pairs.slice(max).join(', ')}><Label variant="soft">+{pairs.length - max}</Label></Tooltip>{/if}
</span>

<style>
	.chips {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}
</style>
