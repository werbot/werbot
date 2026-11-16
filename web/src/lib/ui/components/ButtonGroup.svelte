<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		children?: Snippet;
	}

	let { children }: Props = $props();
</script>

<div class="miso-btn-group">
	{@render children?.()}
</div>

<style>
	.miso-btn-group {
		display: inline-flex;
	}

	.miso-btn-group :global(.miso-btn:focus-visible),
	.miso-btn-group :global(.miso-btn:hover) {
		z-index: 1;
	}

	.miso-btn-group :global(.miso-btn:first-child:not(:last-child)) {
		border-top-right-radius: 0;
		border-bottom-right-radius: 0;
	}

	.miso-btn-group :global(.miso-btn:last-child:not(:first-child)) {
		border-top-left-radius: 0;
		border-bottom-left-radius: 0;
	}

	.miso-btn-group :global(.miso-btn:not(:first-child):not(:last-child)) {
		border-radius: 0;
	}

	/* hairline separator between adjacent solid segments */
	.miso-btn-group :global(.miso-btn--solid:not(:first-child)) {
		box-shadow: 1px 0 0 0 var(--btn-c5) inset;
	}

	.miso-btn-group :global(.miso-btn--invert:not(:first-child)),
	.miso-btn-group :global(.miso-btn--fill:not(:first-child)) {
		box-shadow: 1px 0 0 0 var(--btn-c3) inset;
	}
</style>
