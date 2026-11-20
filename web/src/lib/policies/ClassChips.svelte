<script lang="ts">
	import { Label, type LabelColor } from '#lib/ui/index.js';
	import { classLabel, classTone, type ClassTone } from './classes.js';

	let { classes }: { classes: string[] } = $props();

	const colors: Record<ClassTone, LabelColor> = { neutral: 'coolgrey', info: 'blue', warning: 'orange', alert: 'red' };
</script>

<span class="chips">
	{#each classes as c (c)}<Label variant="soft" color={colors[classTone(c)]}>{classLabel(c)}</Label>{/each}
</span>

<style>
	.chips {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}
</style>
