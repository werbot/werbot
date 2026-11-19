<script lang="ts">
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<main class="public">
	<div class="card">
		<div class="brand">Werbot</div>
		{@render children()}
	</div>
</main>

<style>
	.public {
		display: flex;
		justify-content: center;
		align-items: flex-start;
		box-sizing: border-box;
		min-height: 100vh;
		padding: 64px 16px;
		background: var(--miso-cont-mid);
	}

	.card {
		box-sizing: border-box;
		width: 100%;
		max-width: 400px;
		padding: var(--miso-sp-24);
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
	}

	.brand {
		margin-bottom: var(--miso-sp-16);
		font: var(--miso-font-ui-18-semi);
		color: var(--miso-txt-accent);
	}

	.public :global(form) {
		display: flex;
		flex-direction: column;
		gap: 12px;
		width: 100%;
	}

	.public :global(h1) {
		margin: 0;
		font: var(--miso-font-ui-22-semi);
		color: var(--miso-txt-accent);
	}
</style>
