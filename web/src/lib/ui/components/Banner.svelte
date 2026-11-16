<script lang="ts" module>
	export type BannerTone = 'alert' | 'warning' | 'success' | 'info';
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props {
		tone?: BannerTone;
		/** floating pill (like the offline notice) instead of a block */
		pill?: boolean;
		icon?: IconName;
		children?: Snippet;
	}

	let { tone = 'info', pill = false, icon, children }: Props = $props();

	const defaultIcons: Record<BannerTone, IconName> = {
		alert: 'warning',
		warning: 'warning',
		success: 'circle-check',
		info: 'info'
	};

	let iconName = $derived(icon ?? defaultIcons[tone]);
</script>

<div class={['miso-banner', `miso-banner--${tone}`, pill && 'miso-banner--pill']} role="status">
	<span class="miso-banner__icon"><Icon name={iconName} size={16} /></span>
	<span class="miso-banner__text">{@render children?.()}</span>
</div>

<style>
	.miso-banner {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-radius: var(--miso-radius-08);
		font: var(--miso-font-ui-13-med);
	}

	.miso-banner--pill {
		display: inline-flex;
		padding: 6px 16px;
		border-radius: var(--miso-radius-full);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-major);
	}

	.miso-banner__icon {
		display: inline-flex;
		flex-shrink: 0;
	}

	.miso-banner--alert {
		background: var(--miso-red-50);
		color: var(--miso-txt-alert-major);
		box-shadow: 0 0 0 1px var(--miso-red-300) inset;
	}

	.miso-banner--alert.miso-banner--pill {
		background: var(--miso-cont-top);
		box-shadow:
			0 0 0 1px var(--miso-red-400) inset,
			var(--miso-shadow-cont-major);
	}

	.miso-banner--warning {
		background: var(--miso-amber-50);
		color: var(--miso-txt-notice-major);
		box-shadow: 0 0 0 1px var(--miso-amber-300) inset;
	}

	.miso-banner--success {
		background: var(--miso-lime-50);
		color: var(--miso-green-750);
		box-shadow: 0 0 0 1px var(--miso-green-300) inset;
	}

	.miso-banner--info {
		background: var(--miso-interblue-50);
		color: var(--miso-interblue-900);
		box-shadow: 0 0 0 1px var(--miso-interblue-300) inset;
	}
</style>
