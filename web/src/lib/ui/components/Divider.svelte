<script lang="ts">
	interface Props {
		vertical?: boolean;
		/** spacing around the line in px */
		gap?: number;
	}

	let { vertical = false, gap = 8 }: Props = $props();
</script>

{#if vertical}
	<span class="miso-divider miso-divider--v" style:margin="0 {gap}px" role="separator"></span>
{:else}
	<hr class="miso-divider" style:margin="{gap}px 0" />
{/if}

<style>
	.miso-divider {
		border: 0;
		background: var(--miso-line-tertiary);
	}

	hr.miso-divider {
		height: 1px;
		width: 100%;
	}

	.miso-divider--v {
		display: inline-block;
		width: 1px;
		align-self: stretch;
	}
</style>
