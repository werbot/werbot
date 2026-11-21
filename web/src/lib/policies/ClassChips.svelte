<script lang="ts">
	import { Label } from '#lib/ui/index.js';
	import { classColor, classLabel } from './classes.js';

	let { classes }: { classes: string[] } = $props();
</script>

<span class="chips">
	{#each classes as c (c)}<Label variant="soft" color={classColor(c)}>{classLabel(c)}</Label>{/each}
</span>

<style>
	.chips {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}
</style>
