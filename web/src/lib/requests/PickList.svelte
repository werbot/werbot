<script lang="ts">
	import { Chip, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';

	/** Picked ids as chips plus a select that adds one more (the team-member pattern). */
	let { label, options, ids = $bindable() }: { label: string; options: SelectOption[]; ids: string[] } = $props();

	let pick = $state('');
	const names = $derived(new Map(options.map((o) => [o.value, o.label])));
	const rest = $derived(options.filter((o) => !ids.includes(o.value)));
</script>

<div class="pick">
	{#if ids.length > 0}
		<div class="chips">
			{#each ids as id (id)}
				<Chip deletable ondelete={() => (ids = ids.filter((x) => x !== id))}>{names.get(id) ?? id}</Chip>
			{/each}
		</div>
	{/if}
	<FilledSelect
		{label}
		options={rest}
		bind:value={pick}
		onchange={(v) => {
			ids = [...ids, v];
			pick = '';
		}}
	/>
</div>

<style>
	.pick {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08);
	}
</style>
