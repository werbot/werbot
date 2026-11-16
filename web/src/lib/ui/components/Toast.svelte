<script lang="ts">
	import type { ToastAction, ToastTone } from '../toaster.svelte.js';
	import type { IconName } from '../icons.js';
	import Icon from './Icon.svelte';
	import Button from './Button.svelte';

	interface Props {
		tone?: ToastTone;
		title?: string;
		text?: string;
		icon?: IconName;
		/** countdown ring duration in ms; 0 hides the ring */
		duration?: number;
		/** grouped identical toasts counter; shows a badge when > 1 */
		count?: number;
		action?: ToastAction;
		onclose?: () => void;
	}

	let {
		tone = 'neutral',
		title,
		text,
		icon = 'info',
		duration = 0,
		count = 1,
		action,
		onclose
	}: Props = $props();

	// r=16 ring inside a 36px box
	const circumference = 2 * Math.PI * 16;
</script>

<!-- recreated from the archive .message block: colored border + 8px solid drop -->
<div class={['miso-toast', `miso-toast--${tone}`]}>
	<div class="miso-toast__side">
		{#if duration > 0}
			{#key count}
				<svg class="miso-toast__ring" viewBox="0 0 36 36" aria-hidden="true">
					<circle
						cx="18"
						cy="18"
						r="16"
						fill="none"
						stroke-width="1.5"
						stroke-dasharray={circumference}
						style:animation-duration="{duration}ms"
					/>
				</svg>
			{/key}
		{/if}
		<span class="miso-toast__icon"><Icon name={icon} size={20} /></span>
		{#if count > 1}
			<span class="miso-toast__count">{count > 99 ? '99+' : count}</span>
		{/if}
	</div>
	<div class="miso-toast__main">
		{#if title}
			<span class="miso-toast__title">{title}</span>
		{/if}
		{#if text}
			<span class="miso-toast__text">{text}</span>
		{/if}
		{#if action}
			<span class="miso-toast__options">
				<Button
					scheme="g"
					variant="skeleton"
					size={24}
					onclick={() => {
						action.onclick();
						onclose?.();
					}}
				>
					{action.label}
				</Button>
			</span>
		{/if}
	</div>
	{#if onclose}
		<div class="miso-toast__close">
			<Button scheme="g" variant="ghost" size={28} icon="x" onclick={onclose} aria-label="Dismiss" />
		</div>
	{/if}
</div>

<style>
	.miso-toast {
		display: flex;
		gap: 4px;
		box-sizing: border-box;
		width: 378px;
		max-width: calc(100vw - 48px);
		min-height: 60px;
		padding: 12px;
		border-radius: var(--miso-radius-12);
		border: 1px solid var(--toast-brd);
		/* tone tint composited over opaque white for legibility on any backdrop */
		background:
			linear-gradient(var(--toast-bg), var(--toast-bg)) padding-box,
			var(--miso-cont-top) padding-box;
		box-shadow: 0 8px 0 0 var(--toast-brd);
		color: var(--miso-txt-accent);
	}

	.miso-toast__side {
		position: relative;
		width: 36px;
		height: 36px;
		flex-shrink: 0;
	}

	.miso-toast__ring {
		position: absolute;
		inset: 0;
		width: 36px;
		height: 36px;
		rotate: -90deg;
	}

	.miso-toast__ring circle {
		stroke: var(--toast-accent);
		animation-name: miso-toast-countdown;
		animation-timing-function: linear;
		animation-fill-mode: forwards;
	}

	@keyframes miso-toast-countdown {
		from {
			stroke-dashoffset: 0;
		}
		to {
			stroke-dashoffset: 100.53;
		}
	}

	.miso-toast__icon {
		position: absolute;
		inset: 0;
		display: flex;
		align-items: center;
		justify-content: center;
		color: var(--toast-accent);
	}

	/* grouped duplicates badge */
	.miso-toast__count {
		position: absolute;
		top: -6px;
		right: -8px;
		min-width: 18px;
		height: 18px;
		box-sizing: border-box;
		padding: 0 5px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border-radius: var(--miso-radius-full);
		background: var(--toast-accent);
		/* light tones keep white digits; invert overrides via --toast-count-fg */
		color: var(--toast-count-fg, var(--miso-cont-top));
		font: var(--miso-font-ui-11-semi);
		box-shadow: 0 0 0 2px var(--toast-count-ring, var(--miso-cont-top));
	}

	.miso-toast__main {
		flex: 1;
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 0 8px;
		justify-content: center;
		overflow: hidden;
	}

	.miso-toast__title {
		font: var(--miso-font-ui-15-semi);
		color: var(--toast-accent);
	}

	.miso-toast__text {
		font: var(--miso-font-txt-13-reg);
		white-space: pre-wrap;
		word-break: break-word;
	}

	.miso-toast__options {
		display: flex;
		gap: 4px;
		padding-top: 6px;
	}

	.miso-toast__close {
		flex-shrink: 0;
		margin: -4px -4px 0 0;
	}

	/* tones: border/drop + tinted bg + accent, mapped from message-a/b/g/s/w/d/i */
	.miso-toast--accent {
		--toast-brd: var(--miso-hlt-focus);
		--toast-bg: var(--miso-hlt-easy);
		--toast-accent: var(--miso-btn-a-txt);
	}

	.miso-toast--violet {
		--toast-brd: var(--miso-hlt-b-focus);
		--toast-bg: var(--miso-hlt-b-easy);
		--toast-accent: var(--miso-btn-b-txt);
	}

	.miso-toast--neutral {
		--toast-brd: var(--miso-hlt-g-focus);
		--toast-bg: var(--miso-cont-top);
		--toast-accent: var(--miso-txt-primary);
	}

	.miso-toast--success {
		--toast-brd: var(--miso-txt-new);
		--toast-bg: var(--miso-lime-50);
		--toast-accent: var(--miso-txt-new-major);
	}

	.miso-toast--warning {
		--toast-brd: var(--miso-txt-notice);
		--toast-bg: var(--miso-amber-50);
		--toast-accent: var(--miso-txt-notice-major);
	}

	.miso-toast--alert {
		--toast-brd: var(--miso-hlt-w-focus);
		--toast-bg: var(--miso-hlt-w-easy);
		--toast-accent: var(--miso-txt-alert-major);
	}

	.miso-toast--invert {
		--toast-brd: var(--miso-hlt-g-focus);
		--toast-bg: var(--miso-graphite-960);
		--toast-accent: var(--miso-txt-flip);
		/* white badge on dark toast → dark digits + ring matching toast bg */
		--toast-count-fg: var(--miso-graphite-960);
		--toast-count-ring: var(--miso-graphite-960);
		color: var(--miso-txt-flip);
	}
</style>
