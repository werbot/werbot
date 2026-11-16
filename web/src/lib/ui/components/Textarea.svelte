<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	interface Props extends HTMLTextareaAttributes {
		value?: string;
		label?: string;
		error?: string;
	}

	let { value = $bindable(''), label, error, ...rest }: Props = $props();

	const id = $props.id();
</script>

<div class={['miso-field', error && 'miso-field--error', rest.disabled && 'miso-field--disabled']}>
	{#if label}
		<label class="miso-field__label" for={id}>{label}</label>
	{/if}
	<textarea {id} {...rest} bind:value></textarea>
	{#if error}
		<div class="miso-field__error">{error}</div>
	{/if}
</div>

<style>
	.miso-field {
		display: flex;
		flex-direction: column;
		gap: 4px;
		min-width: 0;
	}

	.miso-field__label {
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-secondary);
	}

	textarea {
		min-height: 72px;
		padding: 6px 8px;
		border: 0;
		border-radius: var(--miso-radius-07);
		background: var(--miso-inp-cont);
		box-shadow: var(--miso-brd-enable);
		font: var(--miso-font-txt-13-reg);
		color: var(--miso-txt-primary);
		resize: vertical;
		transition: box-shadow 120ms ease;
	}

	textarea::placeholder {
		color: var(--miso-txt-muted);
	}

	.miso-field:not(.miso-field--disabled):hover textarea {
		box-shadow: var(--miso-brd-enable-hover);
	}

	textarea:focus,
	.miso-field:not(.miso-field--disabled):hover textarea:focus {
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}

	.miso-field--error textarea,
	.miso-field--error:hover textarea {
		box-shadow: var(--miso-brd-error);
	}

	.miso-field--error textarea:focus {
		box-shadow: var(--miso-brd-error-focus);
	}

	.miso-field__error {
		font: var(--miso-font-ui-11-reg);
		color: var(--miso-txt-alert-major);
	}

	.miso-field--disabled {
		opacity: 0.5;
	}
</style>
