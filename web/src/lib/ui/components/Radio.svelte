<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';

	interface Props extends Omit<HTMLInputAttributes, 'type' | 'size'> {
		group?: HTMLInputAttributes['value'];
		value: HTMLInputAttributes['value'];
		/** control size in px (original is 20) */
		size?: number;
		children?: Snippet;
	}

	let { group = $bindable(), value, size = 20, children, ...rest }: Props = $props();

	let checked = $derived(group === value);
</script>

<label class={['miso-radio', rest.disabled && 'miso-radio--disabled']}>
	<input {...rest} type="radio" {value} bind:group />
	<span class={['miso-radio__dot', checked && 'miso-radio__dot--on']} style:--box-size="{size}px">
		<span class="miso-radio__icon miso-radio__icon--base">
			<Icon name={checked ? 'input-circle-solid-select' : 'input-circle-blank'} {size} />
		</span>
		<span class="miso-radio__icon miso-radio__icon--hover">
			<Icon name={checked ? 'input-circle-solid-select' : 'input-circle-select'} {size} />
		</span>
	</span>
	{#if children}
		<span class="miso-radio__label">{@render children()}</span>
	{/if}
</label>

<style>
	.miso-radio {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		-webkit-tap-highlight-color: transparent;
	}

	.miso-radio--disabled {
		opacity: 0.5;
		cursor: default;
	}

	.miso-radio input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
	}

	.miso-radio__dot {
		position: relative;
		display: inline-flex;
		width: var(--box-size);
		height: var(--box-size);
		flex-shrink: 0;
		color: var(--miso-txt-muted);
		border-radius: var(--miso-radius-full);
		transition: color 120ms ease;
	}

	.miso-radio__dot--on {
		color: var(--miso-hlt-invert);
	}

	.miso-radio__icon {
		display: inline-flex;
	}

	.miso-radio__icon--hover {
		position: absolute;
		inset: 0;
		opacity: 0;
	}

	.miso-radio:hover input:not(:disabled) ~ .miso-radio__dot {
		color: var(--miso-txt-secondary);
	}

	.miso-radio:hover input:checked:not(:disabled) ~ .miso-radio__dot {
		color: var(--miso-hlt-invert-major);
	}

	.miso-radio:hover input:not(:disabled) ~ .miso-radio__dot .miso-radio__icon--base {
		opacity: 0;
	}

	.miso-radio:hover input:not(:disabled) ~ .miso-radio__dot .miso-radio__icon--hover {
		opacity: 1;
	}

	.miso-radio input:focus-visible ~ .miso-radio__dot {
		box-shadow: var(--miso-brd-focus);
	}
</style>
