<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props extends Omit<HTMLInputAttributes, 'size'> {
		value?: string;
		/** floating label text */
		label: string;
		/** compact 36px variant (original .inputBlock__label-h36) */
		compact?: boolean;
		/** multiline: renders a textarea */
		multiline?: boolean;
		rows?: number;
		icon?: IconName;
		error?: boolean;
		/** show remaining characters counter (requires maxlength) */
		counter?: boolean;
	}

	let {
		value = $bindable(''),
		label,
		compact = false,
		multiline = false,
		rows = 3,
		icon,
		error = false,
		counter = false,
		...rest
	}: Props = $props();

	/* the same attrs feed either an input or a textarea */
	let restAny = $derived(rest as Record<string, unknown>);

	let filled = $derived(value !== '' && value != null);
	let left = $derived(
		rest.maxlength != null ? Math.max(0, rest.maxlength - String(value ?? '').length) : 0
	);
</script>

<!-- 1:1 with the original .inputBlock__label (filled floating-label field) -->
<label
	class={[
		'miso-filled',
		compact && 'miso-filled--compact',
		multiline && 'miso-filled--area',
		error && 'miso-filled--error',
		rest.disabled && 'miso-filled--disabled'
	]}
>
	{#if icon}
		<span class="miso-filled__icon"><Icon name={icon} size={20} /></span>
	{/if}
	{#if multiline}
		<textarea {...restAny} bind:value {rows} placeholder={rest.placeholder ?? ' '}></textarea>
	{:else}
		<input {...rest} bind:value placeholder={rest.placeholder ?? ' '} />
	{/if}
	<span class={['miso-filled__label', filled && 'miso-filled__label--up']}>{label}</span>
	{#if counter && rest.maxlength != null}
		<span class="miso-filled__counter">{left}</span>
	{/if}
</label>

<style>
	/* .inputBlock__label{height:60px;border-radius:12px;background:trans-low} */
	.miso-filled {
		position: relative;
		display: flex;
		align-items: center;
		height: 60px;
		margin: 0;
		border-radius: 12px;
		overflow: hidden;
		box-sizing: border-box;
		background: var(--miso-inp-filled);
		transition:
			border-color 0.15s ease-in-out,
			box-shadow 0.15s ease-in-out,
			background 0.15s ease-in-out;
	}

	.miso-filled:focus-within {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-brd-focus);
	}

	.miso-filled--error {
		box-shadow: var(--miso-brd-error);
		background: var(--miso-hlt-w-easy);
	}

	.miso-filled--error:focus-within {
		box-shadow: var(--miso-brd-error-focus);
		background: var(--miso-cont-top);
	}

	.miso-filled--disabled {
		opacity: 0.6;
	}

	.miso-filled__icon {
		display: inline-flex;
		flex-shrink: 0;
		margin-left: 14px;
		color: var(--miso-txt-secondary);
	}

	/* input{padding:25.5px 12px 10.5px 14px;font-size:16px} */
	.miso-filled input,
	.miso-filled textarea {
		width: 100%;
		border: 0;
		background: none;
		padding: 25.5px 12px 10.5px 14px;
		font-family: var(--miso-font-base);
		font-size: 16px;
		line-height: 20px;
		font-weight: 400;
		letter-spacing: -0.02em;
		border-radius: 12px;
		color: var(--miso-txt-accent);
		transition: all 0.15s ease;
		outline: none;
		resize: none;
	}

	.miso-filled__icon ~ input,
	.miso-filled__icon ~ textarea {
		padding-left: 8px;
	}

	.miso-filled--area {
		height: auto;
		align-items: stretch;
	}

	/* .label{top:20px;left:16px;font:16/20;origin 0 0} */
	.miso-filled__label {
		position: absolute;
		top: 20px;
		left: 16px;
		font: 400 1rem/20px var(--miso-font-base);
		color: var(--miso-txt-tertiary);
		pointer-events: none;
		transform-origin: 0 0;
		transform: translate3d(0, 0, 0);
		transition: all 0.2s ease;
		white-space: nowrap;
		overflow: hidden;
		max-width: calc(100% - 32px);
		text-overflow: ellipsis;
	}

	.miso-filled__icon ~ input ~ .miso-filled__label,
	.miso-filled__icon ~ textarea ~ .miso-filled__label {
		left: 42px;
	}

	/* filled/focused: translate3d(0,-12px,0) scale(.75) */
	.miso-filled:focus-within .miso-filled__label,
	.miso-filled__label--up {
		transform: translate3d(0, -12px, 0) scale(0.75);
	}

	/* counter chip, top-right */
	.miso-filled__counter {
		position: absolute;
		top: 4px;
		right: 4px;
		min-width: 20px;
		height: 16px;
		padding: 0 4px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		border-radius: 4px;
		font: 10px/16px var(--miso-font-mono);
		color: var(--miso-txt-tertiary);
	}

	.miso-filled--error .miso-filled__counter {
		background: var(--miso-btn-w-txt);
		color: var(--miso-txt-flip);
		border-radius: 3px 8px 3px 5px;
	}

	/* compact 36px variant: label fades out when the value is set */
	.miso-filled--compact {
		height: 36px;
		border-radius: 8px;
	}

	.miso-filled--compact input {
		padding: 2px 12px;
		font-size: 15px;
		font-weight: 500;
	}

	.miso-filled--compact .miso-filled__label {
		top: 8px;
		left: 12px;
		font: var(--miso-font-ui-13-reg);
	}

	.miso-filled--compact:focus-within .miso-filled__label,
	.miso-filled--compact .miso-filled__label--up {
		transform: translate3d(0, 0, 0) scale(0.75);
		opacity: 0;
	}
</style>
