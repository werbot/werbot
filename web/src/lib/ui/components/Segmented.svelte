<script lang="ts" module>
	import type { IconName } from '../icons.js';

	export interface SegmentedOption {
		value: string;
		label?: string;
		icon?: IconName;
	}
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		options: SegmentedOption[];
		value?: string;
		/** control height in px */
		size?: 24 | 28 | 32;
		/** stretch items to equal widths */
		grow?: boolean;
		/** active item text color, as in the original miso-slider.blue / .violet */
		accent?: 'none' | 'blue' | 'violet';
		onchange?: (value: string) => void;
	}

	let {
		options,
		value = $bindable(options[0]?.value),
		size = 28,
		grow = false,
		accent = 'none',
		onchange
	}: Props = $props();

	let list: HTMLDivElement | undefined = $state();
	let thumb = $state({ left: 0, width: 0 });
	let ready = $state(false);

	function measure() {
		if (!list) return;
		const active = list.querySelector<HTMLButtonElement>('[data-active="true"]');
		if (!active) return;
		thumb = { left: active.offsetLeft, width: active.offsetWidth };
		if (!ready) requestAnimationFrame(() => (ready = true));
	}

	$effect(() => {
		void value;
		void options;
		measure();
	});

	function pick(option: SegmentedOption) {
		value = option.value;
		onchange?.(option.value);
	}
</script>

<svelte:window onresize={measure} />

<div
	class={[
		'miso-segmented',
		`miso-segmented--s${size}`,
		grow && 'miso-segmented--grow',
		accent !== 'none' && `miso-segmented--${accent}`
	]}
	role="tablist"
	bind:this={list}
>
	<span
		class={['miso-segmented__thumb', ready && 'miso-segmented__thumb--animated']}
		style:left="{thumb.left}px"
		style:width="{thumb.width}px"
		aria-hidden="true"
	></span>
	{#each options as option (option.value)}
		<button
			type="button"
			class={['miso-segmented__item', option.value === value && 'miso-segmented__item--active']}
			role="tab"
			aria-selected={option.value === value}
			data-active={option.value === value}
			onclick={() => pick(option)}
		>
			{#if option.icon}<Icon name={option.icon} size={size <= 24 ? 14 : 16} />{/if}
			{#if option.label}<span>{option.label}</span>{/if}
		</button>
	{/each}
</div>

<style>
	.miso-segmented {
		position: relative;
		display: inline-flex;
		gap: 2px;
		padding: 2px;
		box-sizing: border-box;
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-trans-low);
	}

	.miso-segmented--grow {
		display: flex;
	}

	.miso-segmented--grow .miso-segmented__item {
		flex: 1;
		justify-content: center;
	}

	/* sliding white pill behind the active item */
	.miso-segmented__thumb {
		position: absolute;
		top: 2px;
		bottom: 2px;
		border-radius: var(--miso-radius-06);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
	}

	.miso-segmented__thumb--animated {
		transition:
			left 0.25s cubic-bezier(0.3, 0, 0.2, 1),
			width 0.25s cubic-bezier(0.3, 0, 0.2, 1);
	}

	.miso-segmented__item {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 0 8px;
		border: 0;
		border-radius: var(--miso-radius-06);
		background: transparent;
		font: var(--miso-font-ui-11-med);
		color: var(--miso-txt-tertiary);
		white-space: nowrap;
		cursor: pointer;
		transition: color 120ms ease;
	}

	.miso-segmented--s24 .miso-segmented__item { height: 20px; }
	.miso-segmented--s28 .miso-segmented__item { height: 24px; }
	.miso-segmented--s32 .miso-segmented__item { height: 28px; font: var(--miso-font-ui-12-med); }

	.miso-segmented__item:hover:not(.miso-segmented__item--active) {
		color: var(--miso-txt-primary);
	}

	.miso-segmented__item:focus-visible {
		outline: none;
		box-shadow: 0 0 0 2px var(--miso-hlt-focus);
	}

	.miso-segmented__item--active {
		color: var(--miso-txt-primary);
	}

	/* miso-slider.blue a.act{color:txt-btn-a} / .violet — txt-btn-b */
	.miso-segmented--blue .miso-segmented__item--active {
		color: var(--miso-btn-a-txt);
	}

	.miso-segmented--violet .miso-segmented__item--active {
		color: var(--miso-btn-b-txt);
	}
</style>
