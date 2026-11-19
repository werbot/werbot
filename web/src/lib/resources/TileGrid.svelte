<script lang="ts">
	import { Icon, type IconName } from '#lib/ui/index.js';

	export interface Tile {
		value: string;
		label: string;
		icon: IconName;
	}

	interface Props {
		tiles: Tile[];
		label: string;
		onpick: (value: string) => void;
	}

	let { tiles, label, onpick }: Props = $props();
</script>

<div class="grid" role="group" aria-label={label}>
	{#each tiles as tile (tile.value)}
		<button type="button" class="tile" onclick={() => onpick(tile.value)}>
			<Icon name={tile.icon} size={32} />
			<span>{tile.label}</span>
		</button>
	{/each}
</div>

<style>
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
		gap: var(--miso-sp-12);
	}

	.tile {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: var(--miso-sp-08);
		min-height: 104px;
		padding: var(--miso-sp-16);
		border: 1px solid var(--miso-line-secondary);
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-top);
		font: var(--miso-font-ui-13-med);
		color: var(--miso-txt-primary);
		cursor: pointer;
	}

	.tile :global(svg) {
		fill: var(--miso-txt-secondary);
	}

	/* same gold fill and dark text as .miso-btn--solid */
	.tile:hover,
	.tile:focus-visible {
		background: var(--miso-hlt-invert);
		border-color: transparent;
		color: var(--wb-on-brand);
	}

	.tile:hover :global(svg),
	.tile:focus-visible :global(svg) {
		fill: var(--wb-on-brand);
	}

	.tile:active {
		background: var(--miso-hlt-invert-major);
	}

	.tile:focus-visible {
		outline: 2px solid var(--wb-focus);
		outline-offset: 2px;
	}
</style>
