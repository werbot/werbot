<script lang="ts">
	import { Label } from '#lib/ui/index.js';

	let { paths }: { paths: string[] } = $props();
</script>

<span class="chips">
	{#each paths as p (p)}<Label variant="soft"><code>{p}</code></Label>{/each}
</span>

<style>
	.chips {
		display: inline-flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}

	code {
		font-family: var(--miso-font-mono);
		font-size: 12px;
		overflow-wrap: anywhere;
	}
</style>
