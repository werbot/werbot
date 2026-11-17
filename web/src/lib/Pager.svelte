<script lang="ts">
	import { Button } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	// page is 1-based; next is the server's next_page (0: last page).
	let { page = $bindable(), next }: { page: number; next: number } = $props();
</script>

{#if page > 1 || next}
	<nav>
		<Button variant="plain" disabled={page <= 1} onclick={() => (page -= 1)}>{m.pagination_prev()}</Button>
		<Button variant="plain" disabled={!next} onclick={() => (page = next)}>{m.pagination_next()}</Button>
	</nav>
{/if}

<style>
	nav {
		display: flex;
		gap: 8px;
		margin-top: 12px;
	}
</style>
