<script lang="ts">
	import { resolveIcon, type IconDef, type IconName } from '../icons.js';

	interface Props {
		name: IconName;
		/** icon square size in px */
		size?: number;
		/** css color, defaults to currentColor */
		color?: string;
	}

	let { name, size = 20, color }: Props = $props();

	let def: IconDef | undefined = $derived(resolveIcon(name));
</script>

{#if def}
	<svg
		class="miso-icon"
		viewBox={def.viewBox ?? '0 0 20 20'}
		width={size}
		height={size}
		fill={def.colored ? undefined : 'currentColor'}
		style:color
		aria-hidden="true"
		focusable="false"
	>
		<!-- eslint-disable-next-line svelte/no-at-html-tags -- static trusted sprite data -->
		{@html def.body}
	</svg>
{/if}

<style>
	.miso-icon {
		display: inline-block;
		flex-shrink: 0;
		vertical-align: middle;
	}
</style>
