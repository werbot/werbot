<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** <th> cells */
		header?: Snippet;
		/** <tr> rows */
		children?: Snippet;
	}

	let { header, children }: Props = $props();
</script>

<table class="miso-table">
	{#if header}
		<thead>
			<tr>{@render header()}</tr>
		</thead>
	{/if}
	<tbody>
		{@render children?.()}
	</tbody>
</table>

<style>
	.miso-table {
		width: 100%;
		border-collapse: collapse;
	}

	.miso-table :global(th) {
		padding: 6px 8px;
		font: var(--miso-font-caption-11-upper);
		text-transform: uppercase;
		letter-spacing: 0.05em;
		color: var(--miso-txt-tertiary);
		text-align: left;
		border-bottom: 1px solid var(--miso-line-tertiary);
		white-space: nowrap;
	}

	.miso-table :global(td) {
		height: 40px;
		padding: 4px 8px;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		border-bottom: 1px solid var(--miso-line-ghost);
	}

	.miso-table :global(tbody tr) {
		transition: background 80ms ease;
	}

	.miso-table :global(tbody tr:hover) {
		background: var(--miso-hlt-easy);
	}

	.miso-table :global(tbody tr[aria-selected='true']) {
		background: var(--miso-hlt-selected);
	}
</style>
