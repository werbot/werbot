<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		label?: string;
		children?: Snippet;
	}

	let { label, children }: Props = $props();
</script>

<div class="miso-menu-group" role="group">
	{#if label}
		<div class="miso-menu-group__label">{label}</div>
	{/if}
	{@render children?.()}
</div>

<style>
	.miso-menu-group {
		display: flex;
		flex-direction: column;
		gap: 1px;
	}

	.miso-menu-group__label {
		padding: 6px 10px 2px;
		font: var(--miso-font-caption-11-upper);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--miso-txt-tertiary);
	}
</style>
