<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLInputAttributes } from 'svelte/elements';

	interface Props extends Omit<HTMLInputAttributes, 'type' | 'size' | 'color'> {
		checked?: boolean;
		/** on-state track color, as in the original: lime (default), blue, violet */
		color?: 'lime' | 'blue' | 'violet';
		/** show the ON/OFF caption before the track (pass strings to localize) */
		state?: boolean | { on?: string; off?: string };
		children?: Snippet;
	}

	let { checked = $bindable(false), color = 'lime', state = false, children, ...rest }: Props = $props();

	const stateOn = $derived(typeof state === 'object' ? (state.on ?? 'ON') : 'ON');
	const stateOff = $derived(typeof state === 'object' ? (state.off ?? 'OFF') : 'OFF');
</script>

<label class={['miso-toggle', `miso-toggle--${color}`, rest.disabled && 'miso-toggle--disabled']}>
	<input {...rest} type="checkbox" role="switch" bind:checked />
	{#if state}
		<span class="miso-toggle__state">{checked ? stateOn : stateOff}</span>
	{/if}
	<span class="miso-toggle__track">
		<span class="miso-toggle__knob"></span>
	</span>
	{#if children}
		<span class="miso-toggle__label">{@render children()}</span>
	{/if}
</label>

<style>
	.miso-toggle {
		--tg-on: var(--miso-btn-togg-on);
		display: inline-flex;
		align-items: center;
		gap: 8px;
		cursor: pointer;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		-webkit-tap-highlight-color: transparent;
	}

	.miso-toggle--blue {
		--tg-on: var(--miso-hlt-invert);
	}

	.miso-toggle--violet {
		--tg-on: var(--miso-hlt-b-invert);
	}

	.miso-toggle--disabled {
		opacity: 0.5;
		cursor: default;
	}

	.miso-toggle input {
		position: absolute;
		opacity: 0;
		width: 0;
		height: 0;
	}

	/* .sw{width:20px;height:12px;padding:2px;border-radius:8px} => 24x16 total */
	.miso-toggle__track {
		position: relative;
		display: inline-flex;
		width: 24px;
		height: 16px;
		flex-shrink: 0;
		box-sizing: border-box;
		border-radius: var(--miso-radius-full);
		box-shadow: 0 0 0 1px var(--miso-btn-togg-ghost) inset;
		transition: all 0.2s ease;
	}

	/* off: 6px ghost dot; on: 10px white dot on the right */
	.miso-toggle__knob {
		position: absolute;
		left: 5px;
		top: 5px;
		width: 6px;
		height: 6px;
		border-radius: var(--miso-radius-full);
		background: var(--miso-btn-togg-ghost);
		transition: all 0.2s ease;
	}

	.miso-toggle input:checked ~ .miso-toggle__track {
		background: var(--tg-on);
		box-shadow: none;
	}

	.miso-toggle input:checked ~ .miso-toggle__track .miso-toggle__knob {
		left: 11px;
		top: 3px;
		width: 10px;
		height: 10px;
		background: #fff;
	}

	/* .sw_txt:before{uppercase;12-mini-med; off:tertiary, on:track color} */
	.miso-toggle__state {
		font: var(--miso-font-ui-12-med);
		text-transform: uppercase;
		color: var(--miso-txt-tertiary);
		transition: color 0.2s ease;
	}

	.miso-toggle input:checked ~ .miso-toggle__state {
		color: var(--tg-on);
	}

	.miso-toggle input:focus-visible ~ .miso-toggle__track {
		box-shadow: 0 0 0 4px var(--miso-hlt-pressed);
	}
</style>
