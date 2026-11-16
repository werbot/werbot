<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** menu min width in px */
		minWidth?: number;
		children?: Snippet;
	}

	let { minWidth = 200, children }: Props = $props();
</script>

<div class="miso-menu" style:min-width="{minWidth}px" role="menu">
	{@render children?.()}
</div>

<style>
	.miso-menu {
		display: flex;
		flex-direction: column;
		gap: 1px;
		padding: 4px;
		box-sizing: border-box;
		max-width: 320px;
		border-radius: var(--miso-radius-12);
		background: var(--miso-menu-cont);
		box-shadow: var(--miso-shadow-mod-minor);
	}

	.miso-menu :global(.miso-menu-group + .miso-menu-group) {
		margin-top: 4px;
		padding-top: 4px;
		border-top: 1px solid var(--miso-line-tertiary);
	}
</style>
