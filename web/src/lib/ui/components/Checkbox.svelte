<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';
	import Icon from './Icon.svelte';

	interface Props extends Omit<HTMLInputAttributes, 'type' | 'size'> {
		checked?: boolean;
		indeterminate?: boolean;
		/** round task-style checkbox */
		round?: boolean;
		/** box size in px (original is 20) */
		size?: number;
		children?: Snippet;
	}

	let {
		checked = $bindable(false),
		indeterminate = $bindable(false),
		round = false,
		size = 20,
		children,
		...rest
	}: Props = $props();

	// Original miso-checkbox swaps sprite icons per state:
	// blank -> select (hover) -> solid-check / solid-minus
	let icon = $derived(
		round
			? indeterminate
				? 'input-circle-solid-minus'
				: checked
					? 'input-circle-solid-check'
					: 'input-circle-blank'
			: indeterminate
				? 'input-box-solid-minus'
				: checked
					? 'input-box-solid-check'
					: 'input-box-blank'
	);
	let hoverIcon = $derived(
		checked || indeterminate ? icon : round ? 'input-circle-select' : 'input-box-select'
	);
</script>

<label class={['miso-checkbox', rest.disabled && 'miso-checkbox--disabled']}>
	<input {...rest} type="checkbox" bind:checked bind:indeterminate />
	<span
		class={[
			'miso-checkbox__box',
			round && 'miso-checkbox__box--round',
			(checked || indeterminate) && 'miso-checkbox__box--on'
		]}
		style:--box-size="{size}px"
	>
		<span class="miso-checkbox__icon miso-checkbox__icon--base"><Icon name={icon} {size} /></span>
		<span class="miso-checkbox__icon miso-checkbox__icon--hover"><Icon name={hoverIcon} {size} /></span>
	</span>
	{#if children}
		<span class="miso-checkbox__label">{@render children()}</span>
	{/if}
</label>

<style>
	.miso-checkbox {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		cursor: pointer;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		-webkit-tap-highlight-color: transparent;
	}

	.miso-checkbox--disabled {
		opacity: 0.5;
		cursor: default;
	}

	.miso-checkbox input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
	}

	.miso-checkbox__box {
		position: relative;
		display: inline-flex;
		width: var(--box-size);
		height: var(--box-size);
		flex-shrink: 0;
		color: var(--miso-txt-muted);
		border-radius: var(--miso-radius-04);
		transition: color 120ms ease;
	}

	.miso-checkbox__box--round {
		border-radius: var(--miso-radius-full);
	}

	.miso-checkbox__box--on {
		color: var(--miso-hlt-invert);
	}

	.miso-checkbox__icon {
		display: inline-flex;
	}

	.miso-checkbox__icon--hover {
		position: absolute;
		inset: 0;
		opacity: 0;
	}

	.miso-checkbox:hover input:not(:disabled) ~ .miso-checkbox__box {
		color: var(--miso-txt-secondary);
	}

	.miso-checkbox:hover input:checked:not(:disabled) ~ .miso-checkbox__box,
	.miso-checkbox:hover input:indeterminate:not(:disabled) ~ .miso-checkbox__box {
		color: var(--miso-hlt-invert-major);
	}

	.miso-checkbox:hover input:not(:disabled) ~ .miso-checkbox__box .miso-checkbox__icon--base {
		opacity: 0;
	}

	.miso-checkbox:hover input:not(:disabled) ~ .miso-checkbox__box .miso-checkbox__icon--hover {
		opacity: 1;
	}

	.miso-checkbox input:focus-visible ~ .miso-checkbox__box {
		box-shadow: var(--miso-brd-focus);
	}
</style>
