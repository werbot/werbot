<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		label: string;
		disabled?: boolean;
		/** the control: gets the id the label points at */
		children: Snippet<[string]>;
	}

	let { label, disabled = false, children }: Props = $props();

	const id = $props.id();
</script>

<!-- FilledField look for native controls: gray fill, label always floated -->
<div class={['filled', disabled && 'filled--disabled']}>
	{@render children(id)}
	<label class="label" for={id}>{label}</label>
</div>

<style>
	.filled {
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
			box-shadow 0.15s ease-in-out,
			background 0.15s ease-in-out;
	}

	.filled:focus-within {
		background: var(--miso-cont-top);
		box-shadow: var(--miso-brd-focus);
	}

	.filled--disabled {
		opacity: 0.6;
	}

	.filled :global(input) {
		width: 100%;
		border: 0;
		background: none;
		padding: 25.5px 40px 10.5px 14px;
		font-family: var(--miso-font-base);
		font-size: 16px;
		line-height: 20px;
		letter-spacing: -0.02em;
		border-radius: 12px;
		color: var(--miso-txt-accent);
		outline: none;
		appearance: none;
	}

	.label {
		position: absolute;
		top: 20px;
		left: 16px;
		font: 400 1rem/20px var(--miso-font-base);
		color: var(--miso-txt-tertiary);
		pointer-events: none;
		transform-origin: 0 0;
		transform: translate3d(0, -12px, 0) scale(0.75);
		white-space: nowrap;
		overflow: hidden;
		max-width: calc(100% - 56px);
		text-overflow: ellipsis;
	}
</style>
