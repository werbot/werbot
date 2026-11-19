<script lang="ts">
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

<main class="public">
	<div class="brand">
		<img src="/logo.svg" alt="" height="48" />
		<span>Werbot</span>
	</div>
	<div class="card">
		{@render children()}
	</div>
</main>

<style>
	.public {
		display: flex;
		flex-direction: column;
		align-items: center;
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
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--miso-sp-08);
		margin-bottom: var(--miso-sp-24);
		font: var(--miso-font-ui-22-semi);
		color: var(--miso-txt-accent);
	}

	.brand img {
		height: 48px;
		width: auto;
	}

	.public :global(form) {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		width: 100%;
	}

	.public :global(.forgot) {
		align-self: center;
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-link);
		text-decoration: none;
	}

	.public :global(.forgot:hover) {
		text-decoration: underline;
	}

	.public :global(h1) {
		margin: 0;
		font: var(--miso-font-ui-22-semi);
		color: var(--miso-txt-accent);
	}
</style>
