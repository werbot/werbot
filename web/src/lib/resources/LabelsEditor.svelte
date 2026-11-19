<script lang="ts">
	import { Button, FilledField } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { LabelRow } from './labels.js';

	let { rows = $bindable() }: { rows: LabelRow[] } = $props();
</script>

<div class="labels">
	<!-- the add action sits on the title line: one row less in the wizard -->
	<div class="head">
		<span>{m.resources_labels()}</span>
		<Button type="button" variant="plain" size={28} onclick={() => rows.push({ key: '', value: '' })}>{m.resources_label_add()}</Button>
	</div>
	{#each rows as row, i (i)}
		<div class="row">
			<FilledField compact label={m.resources_label_key()} bind:value={row.key} />
			<FilledField compact label={m.resources_label_value()} bind:value={row.value} />
			<Button type="button" scheme="w" variant="plain" onclick={() => rows.splice(i, 1)}>{m.resources_label_remove()}</Button>
		</div>
	{/each}
</div>

<style>
	.labels {
		display: flex;
		flex-direction: column;
		gap: 8px;
	}

	.head {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
</style>
