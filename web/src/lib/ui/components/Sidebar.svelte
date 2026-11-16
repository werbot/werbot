<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** expanded sidebar with visible labels */
		expanded?: boolean;
		/** top area (logo) */
		header?: Snippet;
		/** bottom area (profile, settings) */
		footer?: Snippet;
		children?: Snippet;
	}

	let { expanded = false, header, footer, children }: Props = $props();
</script>

<aside class={['miso-sidebar', expanded && 'miso-sidebar--expanded']}>
	{#if header}
		<div class="miso-sidebar__header">{@render header()}</div>
	{/if}
	<nav class="miso-sidebar__nav miso-scroll">
		{@render children?.()}
	</nav>
	{#if footer}
		<div class="miso-sidebar__footer">{@render footer()}</div>
	{/if}
</aside>

<style>
	.miso-sidebar {
		display: flex;
		flex-direction: column;
		width: 56px;
		height: 100%;
		box-sizing: border-box;
		padding: 8px 8px calc(8px + env(safe-area-inset-bottom, 0px));
		background: var(--miso-sidebar-base);
		overflow: hidden;
		flex-shrink: 0;
		transition: width 150ms ease;
	}

	.miso-sidebar--expanded {
		width: 216px;
	}

	.miso-sidebar__header {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding-bottom: 12px;
	}

	.miso-sidebar__nav {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex: 1;
		overflow-y: auto;
		overflow-x: hidden;
	}

	.miso-sidebar__footer {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding-top: 12px;
	}
</style>
