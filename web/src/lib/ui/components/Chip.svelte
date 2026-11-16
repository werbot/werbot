<script lang="ts" module>
	export type ChipVariant = 'plain' | 'outline' | 'fill' | 'dashed';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import Avatar from './Avatar.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		variant?: ChipVariant;
		/** chip height in px */
		size?: 20 | 24 | 28;
		iconLeft?: IconName;
		/** person chip with letter avatar */
		avatar?: string;
		/** show chevron on the right (opens a menu) */
		chevron?: boolean;
		/** show delete cross; fires ondelete */
		deletable?: boolean;
		disabled?: boolean;
		selected?: boolean;
		onclick?: (event: MouseEvent) => void;
		ondelete?: () => void;
		children?: Snippet;
	}

	let {
		variant = 'outline',
		size = 24,
		iconLeft,
		avatar,
		chevron = false,
		deletable = false,
		disabled = false,
		selected = false,
		onclick,
		ondelete,
		children
	}: Props = $props();

	let iconSize = $derived(size <= 20 ? 12 : 16);
	let avatarSize = $derived(size <= 20 ? 14 : size <= 24 ? 18 : 22);
	let interactive = $derived(!disabled && !!onclick);
</script>

<span
	class={[
		'miso-chip',
		`miso-chip--${variant}`,
		`miso-chip--s${size}`,
		disabled && 'miso-chip--disabled',
		selected && 'miso-chip--selected',
		interactive && 'miso-chip--interactive'
	]}
>
	{#if interactive}
		<button type="button" class="miso-chip__hit" {onclick} {disabled} aria-label="chip"></button>
	{/if}
	{#if avatar !== undefined}
		<Avatar name={avatar} size={avatarSize} />
	{:else if iconLeft}
		<Icon name={iconLeft} size={iconSize} />
	{/if}
	{#if children}
		<span class="miso-chip__text">{@render children()}</span>
	{/if}
	{#if chevron}
		<Icon name="chevron-down" size={iconSize} />
	{/if}
	{#if deletable && !disabled}
		<button
			type="button"
			class="miso-chip__delete"
			onclick={(event) => {
				event.stopPropagation();
				ondelete?.();
			}}
			aria-label="Remove"
		>
			<Icon name="x" size={size <= 20 ? 10 : 12} />
		</button>
	{/if}
</span>

<style>
	.miso-chip {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 2px;
		padding: 0 4px;
		box-sizing: border-box;
		border-radius: var(--miso-radius-08);
		background: transparent;
		color: var(--miso-txt-tertiary);
		transition:
			box-shadow 150ms ease-in-out,
			background 150ms ease-in-out;
	}

	.miso-chip--s20 { height: 20px; }
	.miso-chip--s24 { height: 24px; }
	.miso-chip--s28 { height: 28px; border-radius: 10px; }

	.miso-chip__hit {
		position: absolute;
		inset: 0;
		opacity: 0;
		border: 0;
		padding: 0;
		cursor: pointer;
		border-radius: inherit;
	}

	.miso-chip :global(.miso-avatar) {
		margin-left: -1px;
	}

	.miso-chip__text {
		padding: 0 4px;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-secondary);
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.miso-chip--s28 .miso-chip__text {
		font: var(--miso-font-ui-13-reg);
	}

	.miso-chip--outline {
		box-shadow: 0 0 0 1px var(--miso-line-secondary) inset;
	}

	.miso-chip--fill {
		background: var(--miso-hlt-g-hover);
	}

	.miso-chip--dashed {
		outline: 1px dashed var(--miso-line-primary);
		outline-offset: -1px;
	}

	.miso-chip--interactive:hover:not(.miso-chip--disabled) {
		color: var(--miso-txt-accent);
	}

	.miso-chip--interactive:hover:not(.miso-chip--disabled) .miso-chip__text {
		color: var(--miso-txt-accent);
	}

	.miso-chip--outline.miso-chip--interactive:hover:not(.miso-chip--disabled) {
		box-shadow: 0 0 0 1px var(--miso-line-accent) inset;
	}

	.miso-chip--plain.miso-chip--interactive:hover:not(.miso-chip--disabled) {
		background: var(--miso-hlt-g-hover);
	}

	.miso-chip--fill.miso-chip--interactive:hover:not(.miso-chip--disabled) {
		box-shadow: 0 0 0 1px var(--miso-line-accent) inset;
	}

	.miso-chip--dashed.miso-chip--interactive:hover:not(.miso-chip--disabled) {
		outline-color: var(--miso-line-accent);
	}

	.miso-chip--selected {
		background: var(--miso-hlt-selected);
		box-shadow: none;
	}

	.miso-chip__hit:focus-visible {
		opacity: 1;
		outline: none;
		box-shadow: var(--miso-brd-focus);
		background: transparent;
	}

	.miso-chip__delete {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		align-self: stretch;
		margin-right: -3px;
		padding: 0 4px 0 3px;
		border: 0;
		border-left: 1px solid var(--miso-line-tertiary);
		background: transparent;
		color: var(--miso-txt-tertiary);
		cursor: pointer;
		z-index: 1;
	}

	.miso-chip--plain .miso-chip__delete {
		border-left-color: transparent;
	}

	.miso-chip__delete:hover {
		background: var(--miso-hlt-g-hover);
		color: var(--miso-txt-accent);
	}

	.miso-chip--disabled {
		opacity: 0.6;
	}

	.miso-chip--disabled .miso-chip__text {
		color: var(--miso-txt-muted);
	}
</style>
