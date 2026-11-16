<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		icon?: IconName;
		title?: string;
		/** action buttons */
		actions?: Snippet;
		children?: Snippet;
	}

	let { icon, title, actions, children }: Props = $props();
</script>

<div class="miso-empty">
	{#if icon}
		<span class="miso-empty__icon"><Icon name={icon} size={32} /></span>
	{/if}
	{#if title}
		<div class="miso-empty__title">{title}</div>
	{/if}
	{#if children}
		<div class="miso-empty__text">{@render children()}</div>
	{/if}
	{#if actions}
		<div class="miso-empty__actions">{@render actions()}</div>
	{/if}
</div>

<style>
	.miso-empty {
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		gap: 8px;
		padding: 32px 16px;
		text-align: center;
	}

	.miso-empty__icon {
		color: var(--miso-txt-ghost);
	}

	.miso-empty__title {
		font: var(--miso-font-ui-14-semi);
		color: var(--miso-txt-secondary);
	}

	.miso-empty__text {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-tertiary);
		max-width: 360px;
	}

	.miso-empty__actions {
		display: flex;
		gap: 8px;
		margin-top: 8px;
	}
</style>
