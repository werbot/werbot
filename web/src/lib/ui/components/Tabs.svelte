<script lang="ts" module>
	import type { IconName } from '../icons.js';

	export interface TabOption {
		value: string;
		label: string;
		/** leading icon */
		icon?: IconName;
		/** icon rendered after the label */
		trailingIcon?: IconName;
		/** counter rendered in a bordered pill */
		count?: number;
		/** alert paints the counter red (e.g. overdue) */
		countTone?: 'default' | 'alert';
		/** muted look for tabs with no content (still clickable) */
		empty?: boolean;
		disabled?: boolean;
	}
</script>

<script lang="ts">
	import Icon from './Icon.svelte';

	interface Props {
		options: TabOption[];
		value?: string;
		/** icon size for all options, 20 by default */
		iconSize?: number;
		onchange?: (value: string) => void;
	}

	let { options, value = $bindable(options[0]?.value), iconSize = 20, onchange }: Props = $props();

	let list: HTMLDivElement | undefined = $state();
	let bar = $state({ left: 0, width: 0 });
	let ready = $state(false);

	function measure() {
		if (!list) return;
		const active = list.querySelector<HTMLButtonElement>('[data-active="true"]');
		if (!active) return;
		/* underline is inset by the item padding (8px default, themable) */
		const raw = parseFloat(getComputedStyle(list).getPropertyValue('--miso-tabs-bar-inset'));
		const inset = Number.isFinite(raw) ? raw : 8;
		bar = { left: active.offsetLeft + inset, width: Math.max(0, active.offsetWidth - inset * 2) };
		if (!ready) requestAnimationFrame(() => (ready = true));
	}

	$effect(() => {
		void value;
		void options;
		measure();
	});

	function pick(option: TabOption) {
		if (option.disabled) return;
		value = option.value;
		onchange?.(option.value);
	}
</script>

<svelte:window onresize={measure} />

<div class="miso-tabs" role="tablist" bind:this={list}>
	<span
		class={['miso-tabs__bar', ready && 'miso-tabs__bar--animated']}
		style:left="{bar.left}px"
		style:width="{bar.width}px"
		aria-hidden="true"
	></span>
	{#each options as option (option.value)}
		<button
			type="button"
			role="tab"
			aria-selected={value === option.value}
			class={[
				'miso-tabs__item',
				value === option.value && 'miso-tabs__item--active',
				option.empty && 'miso-tabs__item--empty'
			]}
			data-active={value === option.value}
			disabled={option.disabled}
			onclick={() => pick(option)}
		>
			{#if option.icon}
				<span class="miso-tabs__icon"><Icon name={option.icon} size={iconSize} /></span>
			{/if}
			<span class="miso-tabs__label" data-title={option.label}>{option.label}</span>
			{#if option.trailingIcon}
				<span class="miso-tabs__icon"><Icon name={option.trailingIcon} size={iconSize} /></span>
			{/if}
			{#if option.count !== undefined}
				<span class={['miso-tabs__count', option.countTone === 'alert' && 'miso-tabs__count--alert']}>
					{option.count}
				</span>
			{/if}
		</button>
	{/each}
</div>

<style>
	/*
	 * Themable via CSS vars set on a wrapper (all optional):
	 * --miso-tabs-gap, --miso-tabs-height, --miso-tabs-item-pad, --miso-tabs-font,
	 * --miso-tabs-color, --miso-tabs-color-hover, --miso-tabs-color-active,
	 * --miso-tabs-bar (active underline color), --miso-tabs-bar-inset
	 */
	.miso-tabs {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: var(--miso-tabs-gap, 2px);
	}

	/* single sliding 2px underline, flows between tabs like the Segmented thumb */
	.miso-tabs__bar {
		position: absolute;
		bottom: 0;
		height: 2px;
		border-radius: 1px;
		background: var(--miso-tabs-bar, var(--miso-txt-link));
	}

	.miso-tabs__bar--animated {
		transition:
			left 0.25s cubic-bezier(0.3, 0, 0.2, 1),
			width 0.25s cubic-bezier(0.3, 0, 0.2, 1);
	}

	.miso-tabs__item {
		position: relative;
		display: inline-flex;
		align-items: center;
		gap: 4px;
		min-height: var(--miso-tabs-height, 0);
		padding: var(--miso-tabs-item-pad, 6px 8px);
		box-sizing: border-box;
		border: 0;
		background: none;
		font: var(--miso-tabs-font, var(--miso-font-ui-13-reg));
		color: var(--miso-tabs-color, var(--miso-txt-secondary));
		cursor: pointer;
		-webkit-tap-highlight-color: transparent;
	}

	/* gray hover underline on inactive items */
	.miso-tabs__item::after {
		content: '';
		position: absolute;
		left: var(--miso-tabs-bar-inset, 8px);
		right: var(--miso-tabs-bar-inset, 8px);
		bottom: 0;
		height: 2px;
		border-radius: 1px;
		background: var(--miso-txt-secondary);
		opacity: 0;
		transition: opacity 0.3s ease;
	}

	.miso-tabs__item:hover:not(:disabled):not(.miso-tabs__item--active) {
		color: var(--miso-tabs-color-hover, var(--miso-txt-accent));
	}

	.miso-tabs__item:hover:not(:disabled):not(.miso-tabs__item--active)::after {
		opacity: 1;
	}

	.miso-tabs__item--active {
		color: var(--miso-tabs-color-active, var(--miso-txt-primary));
	}

	.miso-tabs__item--active .miso-tabs__label {
		font-weight: var(--miso-tabs-weight-active, 600);
	}

	/* tabs__item.empty — muted when there is nothing behind the tab */
	.miso-tabs__item--empty:not(.miso-tabs__item--active) {
		color: var(--miso-txt-muted);
	}

	.miso-tabs__item:disabled {
		color: var(--miso-txt-muted);
		cursor: default;
	}

	.miso-tabs__icon {
		display: inline-flex;
		flex-shrink: 0;
	}

	/* reserve bold width to avoid layout shift on activation */
	.miso-tabs__label {
		display: inline-flex;
		flex-direction: column;
	}

	.miso-tabs__label::before {
		content: attr(data-title);
		font-weight: var(--miso-tabs-weight-active, 600);
		height: 0;
		overflow: hidden;
		visibility: hidden;
	}

	.miso-tabs__count {
		min-width: 12px;
		padding: 0 3px;
		border: 1px solid currentColor;
		border-radius: var(--miso-radius-12);
		font: var(--miso-font-ui-10-semi);
		line-height: 14px;
		text-align: center;
		color: var(--miso-txt-tertiary);
	}

	.miso-tabs__count--alert {
		color: var(--miso-txt-alert-major);
	}

	.miso-tabs__item:focus-visible {
		outline: none;
		border-radius: var(--miso-radius-06);
		box-shadow: var(--miso-brd-focus);
	}
</style>
