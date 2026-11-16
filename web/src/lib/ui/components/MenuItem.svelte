<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		icon?: IconName;
		/** icon size, 16 by default */
		iconSize?: number;
		/** right-aligned secondary text (hotkey, meta) */
		hint?: string;
		/** right-aligned custom content (info icon, badge…) */
		trailing?: Snippet;
		selected?: boolean;
		disabled?: boolean;
		/** destructive item */
		danger?: boolean;
		/** shows submenu chevron */
		submenu?: boolean;
		/** renders the item as a link */
		href?: string;
		onclick?: (event: MouseEvent) => void;
		onpointerdown?: (event: PointerEvent) => void;
		children?: Snippet;
	}

	let {
		icon,
		iconSize = 16,
		hint,
		trailing,
		selected = false,
		disabled = false,
		danger = false,
		submenu = false,
		href,
		onclick,
		onpointerdown,
		children
	}: Props = $props();

	const classes = $derived([
		'miso-menu-item',
		selected && 'miso-menu-item--selected',
		danger && 'miso-menu-item--danger'
	]);
</script>

{#snippet inner()}
	{#if icon}
		<span class="miso-menu-item__icon"><Icon name={icon} size={iconSize} /></span>
	{/if}
	<span class="miso-menu-item__text">{@render children?.()}</span>
	{#if hint}
		<span class="miso-menu-item__hint">{hint}</span>
	{/if}
	{#if trailing}
		<span class="miso-menu-item__trailing">{@render trailing()}</span>
	{/if}
	{#if submenu}
		<span class="miso-menu-item__chevron"><Icon name="chevron-right" size={14} /></span>
	{/if}
{/snippet}

{#if href}
	<a class={classes} {href} {onclick} {onpointerdown} role="menuitem">
		{@render inner()}
	</a>
{:else}
	<button type="button" class={classes} {disabled} {onclick} {onpointerdown} role="menuitem">
		{@render inner()}
	</button>
{/if}

<style>
	.miso-menu-item {
		display: flex;
		align-items: center;
		gap: 6px;
		width: 100%;
		min-height: 32px;
		padding: 4px 10px 4px 6px;
		box-sizing: border-box;
		border: 0;
		border-radius: var(--miso-radius-08);
		background: transparent;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		text-align: left;
		text-decoration: none;
		cursor: pointer;
	}

	.miso-menu-item__icon {
		display: inline-flex;
		color: var(--miso-txt-tertiary);
	}

	.miso-menu-item__text {
		flex: 1;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.miso-menu-item__hint {
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.miso-menu-item__trailing {
		display: inline-flex;
		flex-shrink: 0;
		color: var(--miso-txt-tertiary);
	}

	.miso-menu-item__chevron {
		display: inline-flex;
		color: var(--miso-txt-muted);
	}

	.miso-menu-item:hover:not(:disabled) {
		background: var(--miso-menu-item-hover);
	}

	.miso-menu-item:focus-visible {
		outline: none;
		background: var(--miso-menu-item-hover);
		box-shadow: 0 0 0 1px var(--miso-hlt-active) inset;
	}

	.miso-menu-item--selected {
		background: var(--miso-menu-item-selected);
	}

	.miso-menu-item--danger {
		color: var(--miso-txt-alert-major);
	}

	.miso-menu-item--danger .miso-menu-item__icon {
		color: var(--miso-txt-alert);
	}

	.miso-menu-item--danger:hover:not(:disabled) {
		background: var(--miso-menu-item-alert);
	}

	.miso-menu-item:disabled {
		color: var(--miso-txt-muted);
		cursor: default;
	}
</style>
