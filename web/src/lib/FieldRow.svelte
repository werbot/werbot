<script lang="ts">
	import type { Snippet } from 'svelte';

	/** narrow (default): a wide field plus a 160px one (host + port); even: two equal halves */
	let { even = false, children }: { even?: boolean; children: Snippet } = $props();
</script>

<div class={['row', even && 'row--even']}>{@render children()}</div>

<style>
	.row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 160px;
		gap: var(--miso-sp-12);
		align-items: start;
	}

	.row--even {
		grid-template-columns: repeat(2, minmax(0, 1fr));
	}

	@media (max-width: 600px) {
		.row,
		.row--even {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
