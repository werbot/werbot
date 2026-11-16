<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		/** leading 36x36 icon well (icon rendered at 20px, tertiary) */
		icon?: IconName;
		title?: string;
		/** grayed-out card — like a disabled settings section (setupBlock__muted) */
		muted?: boolean;
		/** hint icon after the title (info circle) */
		hint?: boolean;
		/** amber "Premium" chip after the title */
		premium?: boolean;
		/** right side of the header (toggle, buttons) */
		actions?: Snippet;
		/** custom header content instead of icon/title */
		header?: Snippet;
		children?: Snippet;
	}

	let {
		icon,
		title,
		muted = false,
		hint = false,
		premium = false,
		actions,
		header,
		children
	}: Props = $props();

	const hasHeader = $derived(Boolean(icon || title || actions || header));
</script>

<section class={['miso-settings-card', muted && 'miso-settings-card--muted']}>
	{#if hasHeader}
		<header class="miso-settings-card__title">
			{#if header}
				{@render header()}
			{:else}
				{#if icon}
					<span class="miso-settings-card__icon"><Icon name={icon} size={20} /></span>
				{/if}
				{#if title}
					<span class="miso-settings-card__in">
						{title}
						{#if premium}
							<span class="miso-settings-card__premium">Premium</span>
						{/if}
						{#if hint}
							<span class="miso-settings-card__hint"><Icon name="info_circle-line" size={16} /></span>
						{/if}
					</span>
				{/if}
			{/if}
			{#if actions}
				<span class="miso-settings-card__actions">{@render actions()}</span>
			{/if}
		</header>
	{/if}
	{#if children}
		<div class="miso-settings-card__content">{@render children()}</div>
	{/if}
</section>

<style>
	/* .setupBlock{background:cont-top;box-shadow:cont-minor-outline;border-radius:12px} */
	.miso-settings-card {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-12);
		font: var(--miso-font-ui-14-reg);
		color: var(--miso-txt-primary);
		box-sizing: border-box;
	}

	.miso-settings-card--muted {
		background: var(--miso-cont-mid);
	}

	.miso-settings-card__title {
		display: flex;
		align-items: center;
		padding: 10px 12px;
		min-height: 36px;
	}

	/* .setupBlock__title miso-icon{width:36px;height:36px} with a 20px glyph */
	.miso-settings-card__icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		flex-shrink: 0;
		color: var(--miso-txt-tertiary);
	}

	.miso-settings-card__in {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		flex: 1;
		min-width: 0;
		padding: 0 6px;
		font: var(--miso-font-ui-16-semi);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.miso-settings-card--muted .miso-settings-card__in {
		color: var(--miso-txt-tertiary);
	}

	.miso-settings-card__hint {
		display: inline-flex;
		color: var(--miso-txt-tertiary);
		cursor: help;
	}

	/* amber outline chip, as on Security premium features */
	.miso-settings-card__premium {
		padding: 0 8px;
		border-radius: 10px;
		box-shadow: 0 0 0 1px var(--miso-btn-notice-major) inset;
		font: var(--miso-font-ui-10-reg);
		color: var(--miso-btn-notice-major);
		white-space: nowrap;
	}

	.miso-settings-card__actions {
		display: inline-flex;
		align-items: center;
		gap: 8px;
		margin-left: auto;
		flex-shrink: 0;
	}

	.miso-settings-card__content {
		padding: 0 20px 20px;
	}

	.miso-settings-card__content:first-child {
		padding: 20px;
	}
</style>
