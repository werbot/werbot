<script lang="ts">
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<main class="public">
	{@render children()}
</main>

<style>
	.public {
		display: flex;
		justify-content: center;
		padding: 64px 16px;
	}

	.public :global(form) {
		display: flex;
		flex-direction: column;
		gap: 12px;
		width: 100%;
		max-width: 360px;
	}

	.public :global(h1) {
		margin: 0;
		font: var(--miso-font-ui-14-med);
	}
</style>
