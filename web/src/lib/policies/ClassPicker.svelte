<script lang="ts">
	import { Checkbox } from '#lib/ui/index.js';
	import { CLASSES, classLabel } from './classes.js';

	/** One checkbox per command class. */
	let { selected = $bindable() }: { selected: string[] } = $props();

	const toggle = (c: string, on: boolean) => (selected = on ? [...selected, c] : selected.filter((x) => x !== c));
</script>

<div class="classes">
	{#each CLASSES as c (c)}
		<Checkbox checked={selected.includes(c)} onchange={(e) => toggle(c, e.currentTarget.checked)}>{classLabel(c)}</Checkbox>
	{/each}
</div>

<style>
	.classes {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
		gap: var(--miso-sp-08);
	}
</style>
