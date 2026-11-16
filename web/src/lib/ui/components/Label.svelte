<script lang="ts" module>
	export type LabelColor =
		| 'none'
		| 'lightgrey'
		| 'coolgrey'
		| 'lightpink'
		| 'pink'
		| 'red'
		| 'yellow'
		| 'yelloworange'
		| 'orange'
		| 'yellowgreen'
		| 'green'
		| 'darkgreen'
		| 'lightaqua'
		| 'aqua'
		| 'darkaqua'
		| 'lightblue'
		| 'blue'
		| 'darkblue'
		| 'lightindigo'
		| 'indigo'
		| 'darkindigo'
		| 'beige'
		| 'brown'
		| 'darkbrown';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		color?: LabelColor;
		/** solid — full color pill, soft — 20% tinted background */
		variant?: 'solid' | 'soft';
		size?: 16 | 20 | 24;
		onclick?: (event: MouseEvent) => void;
		children?: Snippet;
	}

	let { color = 'none', variant = 'solid', size = 20, onclick, children }: Props = $props();

	const darkText = new Set([
		'darkgreen',
		'darkaqua',
		'darkblue',
		'darkindigo',
		'brown',
		'darkbrown',
		'red',
		'indigo'
	]);

	let flip = $derived(variant === 'solid' && darkText.has(color));
</script>

<svelte:element
	this={onclick ? 'button' : 'span'}
	class={[
		'miso-label',
		`miso-label--${variant}`,
		`miso-label--s${size}`,
		flip && 'miso-label--flip',
		onclick && 'miso-label--interactive'
	]}
	style:--lbl-color="var(--miso-lblc-{color})"
	{onclick}
	role={onclick ? undefined : 'status'}
>
	{@render children?.()}
</svelte:element>

<style>
	.miso-label {
		display: inline-flex;
		align-items: center;
		padding: 0 8px;
		border: 0;
		border-radius: var(--miso-radius-06);
		font: var(--miso-font-ui-11-med);
		color: var(--miso-lblc-txt-accent);
		white-space: nowrap;
		vertical-align: middle;
		box-sizing: border-box;
	}

	.miso-label--s16 { height: 16px; padding: 0 6px; border-radius: var(--miso-radius-04); }
	.miso-label--s20 { height: 20px; }
	.miso-label--s24 { height: 24px; font: var(--miso-font-ui-12-med); border-radius: var(--miso-radius-07); }

	.miso-label--solid {
		background: var(--lbl-color);
	}

	.miso-label--flip {
		color: var(--miso-light-a95);
	}

	.miso-label--soft {
		background: color-mix(in srgb, var(--lbl-color) 20%, transparent);
		color: var(--miso-lblc-txt-primary);
	}

	.miso-label--interactive {
		cursor: pointer;
	}

	.miso-label--interactive:hover {
		filter: brightness(0.95);
	}

	.miso-label--interactive:focus-visible {
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}
</style>
