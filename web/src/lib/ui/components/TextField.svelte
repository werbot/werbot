<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';
	import type { IconName } from '../icons.js';

	interface Props extends Omit<HTMLInputAttributes, 'size'> {
		value?: string;
		label?: string;
		hint?: string;
		error?: string;
		iconLeft?: IconName;
		/** control height */
		size?: 28 | 32 | 36;
		/** show remaining characters when maxlength is set */
		counter?: boolean;
	}

	let {
		value = $bindable(''),
		label,
		hint,
		error,
		iconLeft,
		size = 32,
		counter = false,
		...rest
	}: Props = $props();

	const id = $props.id();

	let remaining = $derived(
		typeof rest.maxlength === 'number' ? rest.maxlength - value.length : null
	);
</script>

<div class={['miso-field', error && 'miso-field--error', rest.disabled && 'miso-field--disabled']}>
	{#if label || (counter && remaining !== null)}
		<div class="miso-field__top">
			{#if label}<label class="miso-field__label" for={id}>{label}</label>{/if}
			{#if counter && remaining !== null}
				<span class="miso-field__counter">{remaining}</span>
			{/if}
		</div>
	{/if}
	<div class={['miso-field__control', `miso-field__control--s${size}`]}>
		{#if iconLeft}
			<span class="miso-field__icon"><Icon name={iconLeft} size={16} /></span>
		{/if}
		<input {id} {...rest} bind:value />
	</div>
	{#if error}
		<div class="miso-field__error">{error}</div>
	{:else if hint}
		<div class="miso-field__hint">{hint}</div>
	{/if}
</div>

<style>
	.miso-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}

	.miso-field__top {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}

	.miso-field__label {
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-secondary);
	}

	.miso-field__counter {
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-muted);
	}

	.miso-field__control {
		display: flex;
		align-items: center;
		background: var(--miso-inp-cont);
		border-radius: var(--miso-radius-07);
		box-shadow: var(--miso-brd-enable);
		transition: box-shadow 120ms ease;
	}

	.miso-field__control--s28 { height: 28px; }
	.miso-field__control--s32 { height: 32px; }
	.miso-field__control--s36 { height: 36px; }

	.miso-field__icon {
		display: inline-flex;
		padding-left: 8px;
		color: var(--miso-txt-tertiary);
	}

	.miso-field__control input {
		flex: 1;
		min-width: 0;
		height: 100%;
		padding: 0 8px;
		border: 0;
		background: transparent;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
	}

	.miso-field__control input::placeholder {
		color: var(--miso-txt-muted);
	}

	.miso-field__control input:focus {
		outline: none;
	}

	.miso-field__control input::selection {
		background: var(--miso-inp-selection);
	}

	.miso-field:not(.miso-field--disabled):hover .miso-field__control {
		box-shadow: var(--miso-brd-enable-hover);
	}

	.miso-field__control:focus-within,
	.miso-field:not(.miso-field--disabled):hover .miso-field__control:focus-within {
		box-shadow: var(--miso-brd-focus);
	}

	.miso-field--error .miso-field__control,
	.miso-field--error:hover .miso-field__control {
		box-shadow: var(--miso-brd-error);
	}

	.miso-field--error .miso-field__control:focus-within {
		box-shadow: var(--miso-brd-error-focus);
	}

	.miso-field__hint {
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-tertiary);
	}

	.miso-field__error {
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-alert-major);
	}

	.miso-field--disabled {
		opacity: 0.5;
	}
</style>
