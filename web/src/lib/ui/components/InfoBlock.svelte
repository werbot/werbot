<script lang="ts" module>
	/** matches setupBlock__info-* modifiers: a — blue, b — violet, new — fresh green */
	export type InfoBlockTone = 'default' | 'a' | 'b' | 'new' | 'warning' | 'notice';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		tone?: InfoBlockTone;
		icon?: IconName;
		title?: string;
		/** right side or bottom action (button) */
		actions?: Snippet;
		/** put actions under the text, indented to the title (planPage style) */
		actionsBelow?: boolean;
		children?: Snippet;
	}

	let {
		tone = 'default',
		icon = 'info_circle-line',
		title,
		actions,
		actionsBelow = false,
		children
	}: Props = $props();
</script>

<div class={['miso-info-block', tone !== 'default' && `miso-info-block--${tone}`]} role="status">
	<div class="miso-info-block__wrapper">
		{#if title}
			<div class="miso-info-block__title">
				<span class="miso-info-block__icon"><Icon name={icon} size={20} /></span>
				<span class="miso-info-block__in">{title}</span>
			</div>
		{/if}
		{#if children}
			<div class="miso-info-block__text">{@render children()}</div>
		{/if}
		{#if actions && actionsBelow}
			<div class="miso-info-block__actions miso-info-block__actions--below">{@render actions()}</div>
		{/if}
	</div>
	{#if actions && !actionsBelow}
		<div class="miso-info-block__actions">{@render actions()}</div>
	{/if}
</div>

<style>
	/* .setupBlock__info{padding:12px;border:1px solid line-ghost;border-radius:12px} */
	.miso-info-block {
		display: flex;
		flex-direction: row;
		justify-content: space-between;
		width: 100%;
		padding: 12px;
		box-sizing: border-box;
		border: 1px solid var(--miso-line-ghost);
		border-radius: var(--miso-radius-12);
		font: var(--miso-font-ui-14-reg);
		color: var(--miso-txt-primary);
	}

	.miso-info-block__wrapper {
		display: flex;
		flex-direction: column;
		flex: 1;
		min-width: 0;
	}

	.miso-info-block__title {
		display: flex;
		align-items: center;
	}

	.miso-info-block__icon {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		flex-shrink: 0;
		color: var(--miso-txt-tertiary);
	}

	.miso-info-block__in {
		padding: 8px 6px;
		font: var(--miso-font-ui-16-semi);
	}

	/* .setupBlock__indent{padding:4px 0 8px 42px} — text aligned under the title */
	.miso-info-block__text {
		padding: 0 0 4px 42px;
		max-width: 666px;
		color: var(--miso-txt-accent);
	}

	/* no title above: nothing to indent under */
	.miso-info-block__text:first-child {
		padding: 8px 6px;
	}

	.miso-info-block__actions {
		display: flex;
		align-items: flex-start;
		gap: 8px;
		flex-shrink: 0;
		padding: 8px 6px;
	}

	.miso-info-block__actions--below {
		padding: 12px 0 8px 42px;
	}

	.miso-info-block--a {
		border-color: transparent;
		background: var(--miso-hlt-easy);
	}

	.miso-info-block--a .miso-info-block__in,
	.miso-info-block--a .miso-info-block__icon {
		color: var(--miso-txt-btn-a);
	}

	/* info-b: violet banner from Payment Plan */
	.miso-info-block--b {
		border-color: transparent;
		background: var(--miso-hlt-b-hover);
	}

	.miso-info-block--b .miso-info-block__in,
	.miso-info-block--b .miso-info-block__icon {
		color: var(--miso-txt-btn-b);
	}

	.miso-info-block--new {
		border-color: transparent;
		background: var(--miso-hlt-alt-fresh);
	}

	.miso-info-block--new .miso-info-block__icon {
		color: var(--miso-txt-new-major);
	}

	.miso-info-block--warning .miso-info-block__in,
	.miso-info-block--warning .miso-info-block__icon {
		color: var(--miso-txt-alert-major);
	}

	.miso-info-block--notice .miso-info-block__in,
	.miso-info-block--notice .miso-info-block__icon {
		color: var(--miso-txt-notice-major);
	}
</style>
