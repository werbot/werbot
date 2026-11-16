<script lang="ts" module>
	export type ButtonScheme = 'a' | 'b' | 'g' | 'w' | 'l';
	export type ButtonVariant =
		| 'solid'
		| 'invert'
		| 'fill'
		| 'outline'
		| 'plain'
		| 'skeleton'
		| 'ghost';
	export type ButtonSize = 16 | 20 | 24 | 28 | 32 | 36 | 40 | 48;
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props extends HTMLButtonAttributes {
		/** color scheme: a — blue, b — violet, g — graphite, w — warning, l — light (on dark) */
		scheme?: ButtonScheme;
		variant?: ButtonVariant;
		/** control height in px */
		size?: ButtonSize;
		iconLeft?: IconName;
		iconRight?: IconName;
		/** icon-only square button */
		icon?: IconName;
		/** pressed/selected state for toggle-like buttons */
		selected?: boolean;
		children?: Snippet;
	}

	let {
		scheme = 'a',
		variant = 'fill',
		size = 32,
		iconLeft,
		iconRight,
		icon,
		selected = false,
		children,
		...rest
	}: Props = $props();

	/* original btn-box-small sizes: 16→14, 20/24→16, 28..40→20, 48→24 */
	let iconSize = $derived(size <= 16 ? 14 : size <= 24 ? 16 : size >= 48 ? 24 : 20);
</script>

<button
	{...rest}
	class={[
		'miso-btn',
		`miso-btn--${scheme}`,
		`miso-btn--${variant}`,
		`miso-btn--s${size}`,
		icon && 'miso-btn--box',
		selected && 'miso-btn--selected',
		rest.class
	]}
>
	{#if icon}
		<Icon name={icon} size={iconSize} />
	{:else}
		{#if iconLeft}<Icon name={iconLeft} size={iconSize} />{/if}
		{#if children}<span class="miso-btn__label">{@render children()}</span>{/if}
		{#if iconRight}<Icon name={iconRight} size={iconSize} />{/if}
	{/if}
</button>

<style>
	/* 1:1 with the original .btn element */
	.miso-btn {
		appearance: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		flex-wrap: nowrap;
		border: 0;
		margin: 0;
		cursor: pointer;
		user-select: none;
		text-decoration: none;
		white-space: nowrap;
		vertical-align: middle;
		border-radius: 7px;
		background: transparent;
		font: var(--miso-font-ui-13-med);
		transition: box-shadow 80ms ease;
		box-sizing: border-box;
	}

	.miso-btn:focus-visible {
		outline: none;
	}

	.miso-btn__label {
		padding: 0 var(--btn-label-pad, 4px);
		overflow: hidden;
		text-overflow: ellipsis;
	}

	/* ------------------------------------------------------------ */
	/* Sizes — heights, paddings and radii follow the original scale */
	/* (16/20/24 → r5, 28 → r6, 32 → r7, 36 → r8, 40 → r9, 48 → r10) */
	/* ------------------------------------------------------------ */
	.miso-btn--s16 { height: 16px; padding: 0 2px; font: var(--miso-font-ui-11-med); border-radius: 5px; }
	.miso-btn--s20 { height: 20px; padding: 0 2px; font: var(--miso-font-ui-11-med); border-radius: 5px; }
	.miso-btn--s24 { height: 24px; padding: 0 4px; font: var(--miso-font-ui-11-med); border-radius: 5px; }
	.miso-btn--s28 { height: 28px; padding: 0 4px; font: var(--miso-font-ui-12-med); border-radius: 6px; }
	.miso-btn--s32 { height: 32px; padding: 0 6px; --btn-label-pad: 6px; border-radius: 7px; }
	.miso-btn--s36 { height: 36px; padding: 0 8px; --btn-label-pad: 8px; border-radius: 8px; }
	.miso-btn--s40 { height: 40px; padding: 0 10px; font: var(--miso-font-ui-14-med); --btn-label-pad: 8px; border-radius: 9px; }
	.miso-btn--s48 { height: 48px; padding: 0 12px; font: var(--miso-font-ui-14-med); --btn-label-pad: 8px; border-radius: 10px; }

	.miso-btn--box.miso-btn--s16 { width: 16px; padding: 0; }
	.miso-btn--box.miso-btn--s20 { width: 20px; padding: 0; }
	.miso-btn--box.miso-btn--s24 { width: 24px; padding: 0; }
	.miso-btn--box.miso-btn--s28 { width: 28px; padding: 0; }
	.miso-btn--box.miso-btn--s32 { width: 32px; padding: 0; }
	.miso-btn--box.miso-btn--s36 { width: 36px; padding: 0; }
	.miso-btn--box.miso-btn--s40 { width: 40px; padding: 0; }
	.miso-btn--box.miso-btn--s48 { width: 48px; padding: 0; }

	/* ------------------------------------------------------------ */
	/* Schemes: c1 hover, c2 selected, c3 pressed, c4 invert,        */
	/* c5 invert-major, txt — scheme text color                      */
	/* ------------------------------------------------------------ */
	.miso-btn--a {
		--btn-c1: var(--miso-hlt-hover);
		--btn-c2: var(--miso-hlt-selected);
		--btn-c3: var(--miso-hlt-pressed);
		--btn-c4: var(--miso-hlt-invert);
		--btn-c5: var(--miso-hlt-invert-major);
		--btn-txt: var(--miso-btn-a-txt);
	}

	.miso-btn--b {
		--btn-c1: var(--miso-hlt-b-hover);
		--btn-c2: var(--miso-hlt-b-selected);
		--btn-c3: var(--miso-hlt-b-pressed);
		--btn-c4: var(--miso-hlt-b-invert);
		--btn-c5: var(--miso-hlt-b-invert-major);
		--btn-txt: var(--miso-btn-b-txt);
	}

	.miso-btn--g {
		--btn-c1: var(--miso-hlt-g-hover);
		--btn-c2: var(--miso-hlt-g-selected);
		--btn-c3: var(--miso-hlt-g-pressed);
		--btn-c4: var(--miso-hlt-g-invert);
		--btn-c5: var(--miso-hlt-g-invert-major);
		--btn-txt: var(--miso-btn-g-txt);
	}

	.miso-btn--w {
		--btn-c1: var(--miso-hlt-w-hover);
		--btn-c2: var(--miso-hlt-w-selected);
		--btn-c3: var(--miso-hlt-w-pressed);
		--btn-c4: var(--miso-hlt-w-invert);
		--btn-c5: var(--miso-hlt-w-invert-major);
		--btn-txt: var(--miso-btn-w-txt);
	}

	.miso-btn--l {
		--btn-c1: var(--miso-light-a20);
		--btn-c2: var(--miso-light-a90);
		--btn-c3: var(--miso-light-a70);
		--btn-c4: #ffffff;
		--btn-c5: var(--miso-light-a90);
		--btn-txt: var(--miso-graphite-990);
	}

	/* selected / focus-visible ring, 1:1 with the original:
	   box-shadow: 0 0 0 2px cont-top, 0 0 0 4px c4 */
	.miso-btn--selected,
	.miso-btn:focus-visible {
		--btn-ring: 0 0 0 2px var(--miso-cont-top), 0 0 0 4px var(--btn-c4);
	}

	/* ------------------------------------------------------------ */
	/* Variants                                                       */
	/* ------------------------------------------------------------ */

	/* solid — filled with the scheme accent (font bold) */
	.miso-btn--solid {
		font-weight: 700;
		background: var(--btn-c4);
		color: var(--miso-txt-flip);
	}
	.miso-btn--l.miso-btn--solid { color: var(--btn-txt); }
	.miso-btn--solid:hover:not(:disabled) { background: var(--btn-c5); }
	.miso-btn--solid:focus-visible {
		box-shadow: 0 0 0 1px var(--btn-c5), 0 0 0 4px var(--btn-c3);
	}
	.miso-btn--solid.miso-btn--selected {
		background: var(--btn-c5);
		box-shadow: var(--btn-ring);
	}

	/* invert — translucent pressed background, accent text; fills on hover */
	.miso-btn--invert {
		font-weight: 600;
		background: var(--btn-c3);
		color: var(--btn-txt);
	}
	.miso-btn--g.miso-btn--invert { color: var(--miso-hlt-g-invert-major); }
	.miso-btn--invert:hover:not(:disabled) {
		background: var(--btn-c4);
		color: var(--miso-txt-flip);
	}
	.miso-btn--invert:focus-visible,
	.miso-btn--invert.miso-btn--selected {
		background: var(--btn-c4);
		color: var(--miso-txt-flip);
		box-shadow: var(--btn-ring);
	}
	.miso-btn--l.miso-btn--invert:hover:not(:disabled),
	.miso-btn--l.miso-btn--invert:focus-visible { color: var(--btn-txt); }

	/* fill — light translucent background */
	.miso-btn--fill {
		background: var(--btn-c1);
		color: var(--btn-txt);
	}
	.miso-btn--g.miso-btn--fill { color: var(--miso-hlt-g-invert); }
	.miso-btn--fill:hover:not(:disabled) { background: var(--btn-c2); }
	.miso-btn--fill:focus-visible,
	.miso-btn--fill.miso-btn--selected {
		background: var(--btn-c2);
		box-shadow: var(--btn-ring);
	}

	/* outline — 1px inner border in scheme color */
	.miso-btn--outline {
		color: var(--btn-txt);
		box-shadow: 0 0 0 1px var(--btn-txt) inset;
	}
	.miso-btn--outline:hover:not(:disabled) { background: var(--btn-c1); }
	.miso-btn--g.miso-btn--outline:hover:not(:disabled) { color: var(--miso-hlt-g-invert-major); }
	.miso-btn--outline:focus-visible,
	.miso-btn--outline.miso-btn--selected {
		background: var(--btn-c1);
		box-shadow: var(--btn-ring);
	}
	.miso-btn--outline:disabled { box-shadow: 0 0 0 1px var(--miso-line-secondary) inset; }

	/* plain — text only, background appears on hover */
	.miso-btn--plain {
		color: var(--btn-txt);
	}
	.miso-btn--g.miso-btn--plain:hover:not(:disabled) { color: var(--miso-hlt-g-invert); }
	.miso-btn--plain:hover:not(:disabled) { background: var(--btn-c1); }
	.miso-btn--plain:focus-visible,
	.miso-btn--plain.miso-btn--selected {
		background: var(--btn-c1);
		box-shadow: var(--btn-ring);
	}

	/* skeleton — neutral text with hairline, tints on hover */
	.miso-btn--skeleton {
		font-weight: 400;
		color: var(--miso-txt-primary);
		box-shadow: 0 0 0 1px var(--miso-line-secondary) inset;
	}
	.miso-btn--skeleton:hover:not(:disabled) {
		background: var(--btn-c1);
		box-shadow: 0 0 0 1px var(--btn-txt) inset;
		color: var(--btn-txt);
	}
	.miso-btn--g.miso-btn--skeleton:hover:not(:disabled) {
		color: var(--miso-hlt-g-invert-major);
		box-shadow: 0 0 0 1px var(--miso-line-accent) inset;
	}
	.miso-btn--skeleton:focus-visible,
	.miso-btn--skeleton.miso-btn--selected {
		background: var(--btn-c1);
		color: var(--miso-txt-accent);
		box-shadow: var(--btn-ring);
	}

	/* ghost — neutral text, no border */
	.miso-btn--ghost {
		font-weight: 400;
		color: var(--miso-txt-primary);
	}
	.miso-btn--ghost:hover:not(:disabled) {
		background: var(--btn-c1);
		color: var(--btn-txt);
	}
	.miso-btn--g.miso-btn--ghost:hover:not(:disabled) { color: var(--miso-txt-accent); }
	.miso-btn--ghost:focus-visible,
	.miso-btn--ghost.miso-btn--selected {
		background: var(--btn-c1);
		color: var(--btn-txt);
		box-shadow: var(--btn-ring);
	}
	.miso-btn--l.miso-btn--ghost { color: var(--miso-light-a80); }

	.miso-btn:disabled {
		opacity: 0.5;
		cursor: default;
	}
</style>
