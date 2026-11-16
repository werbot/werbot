<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		/** dark variant used inside tooltips */
		onDark?: boolean;
		children?: Snippet;
	}

	let { onDark = false, children }: Props = $props();
</script>

<kbd class={['miso-kbd', onDark && 'miso-kbd--dark']}>{@render children?.()}</kbd>

<style>
	.miso-kbd {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-width: 20px;
		padding: 0 6px;
		height: 20px;
		box-sizing: border-box;
		border-radius: var(--miso-radius-04);
		border: 1px solid var(--miso-line-secondary);
		background: var(--miso-cont-mid);
		font: var(--miso-font-ui-11-med);
		color: var(--miso-txt-secondary);
		text-transform: capitalize;
	}

	.miso-kbd--dark {
		border-color: var(--miso-light-a20);
		background: var(--miso-light-a10);
		color: var(--miso-light-a90);
	}
</style>
