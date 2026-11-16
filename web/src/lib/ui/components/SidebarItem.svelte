<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import Badge from './Badge.svelte';
	import type { IconName } from '../icons.js';
	import type { BadgeTone } from './Badge.svelte';

	interface Props {
		icon: IconName;
		active?: boolean;
		/** counter badge */
		count?: number;
		countTone?: BadgeTone;
		onclick?: (event: MouseEvent) => void;
		children?: Snippet;
	}

	let { icon, active = false, count, countTone = 'base', onclick, children }: Props = $props();
</script>

<button type="button" class={['miso-sidebar-item', active && 'miso-sidebar-item--active']} {onclick}>
	<span class="miso-sidebar-item__icon">
		<Icon name={icon} size={20} />
	</span>
	{#if children}
		<span class="miso-sidebar-item__label">{@render children()}</span>
	{/if}
	{#if count !== undefined && count > 0}
		<span class="miso-sidebar-item__badge">
			<Badge tone={countTone}>{count > 99 ? '99+' : count}</Badge>
		</span>
	{/if}
</button>

<style>
	.miso-sidebar-item {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		height: 40px;
		padding: 0 10px;
		box-sizing: border-box;
		border: 0;
		border-radius: var(--miso-radius-08);
		background: transparent;
		color: var(--miso-sidebar-ico-base);
		font: var(--miso-font-ui-13-med);
		cursor: pointer;
		white-space: nowrap;
		transition:
			background 120ms ease,
			color 120ms ease;
	}

	.miso-sidebar-item__icon {
		display: inline-flex;
		flex-shrink: 0;
	}

	.miso-sidebar-item__label {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		text-align: left;
		color: var(--miso-titan-300);
	}

	.miso-sidebar-item__badge {
		position: absolute;
		top: 4px;
		right: 4px;
	}

	.miso-sidebar-item:hover {
		background: var(--miso-sidebar-hover);
		color: var(--miso-sidebar-ico-hover);
	}

	.miso-sidebar-item:focus-visible {
		outline: none;
		box-shadow: 0 0 0 2px var(--miso-interblue-500) inset;
	}

	.miso-sidebar-item--active {
		background: var(--miso-sidebar-active);
		color: var(--miso-sidebar-ico-active);
	}
</style>
