<script lang="ts">
	import { Dropdown, Icon, Menu, MenuItem, type SelectOption } from '#lib/ui/index.js';

	interface Props {
		label: string;
		options: SelectOption[];
		value?: string;
		disabled?: boolean;
		onchange?: (value: string) => void;
	}

	let { label, options, value = $bindable(''), disabled = false, onchange }: Props = $props();

	const id = $props.id();
	let open = $state(false);
	let width = $state(0);
	let current = $derived(options.find((o) => o.value === value));

	function pick(option: SelectOption) {
		open = false;
		if (option.value === value) return;
		value = option.value;
		onchange?.(value);
	}

	// Escape closes the menu only: the drawer or modal around it listens later, on the bubble phase.
	function onkeydown(event: KeyboardEvent) {
		if (open && event.key === 'Escape') {
			event.stopImmediatePropagation();
			open = false;
		}
	}
</script>

<svelte:window onkeydowncapture={onkeydown} />

<div class={['fsel', current?.icon && 'fsel--icon', disabled && 'fsel--disabled']} bind:clientWidth={width}>
	<Dropdown bind:open>
		{#snippet trigger({ toggle })}
			<button {id} type="button" class="fsel__control" onclick={toggle} {disabled}>
				{#if current?.icon}<span class="fsel__icon"><Icon name={current.icon} size={20} /></span>{/if}
				<span class="fsel__value">{current?.label ?? ''}</span>
				<span class="fsel__chevron"><Icon name="chevron-down" size={16} /></span>
			</button>
		{/snippet}
		<div class="fsel__list">
			<Menu minWidth={width}>
				{#each options as option (option.value)}
					<MenuItem icon={option.icon} selected={option.value === value} onclick={() => pick(option)}>{option.label}</MenuItem>
				{/each}
			</Menu>
		</div>
	</Dropdown>
	<label class="fsel__label" for={id}>{label}</label>
</div>

<style>
	.fsel,
	.fsel :global(.miso-dropdown) {
		position: relative;
		display: flex;
		width: 100%;
	}

	.fsel__control {
		position: relative;
		display: flex;
		align-items: center;
		gap: 8px;
		width: 100%;
		height: 60px;
		padding: 22px 12px 8px 14px;
		box-sizing: border-box;
		border: 0;
		border-radius: 12px;
		background: var(--miso-inp-filled);
		text-align: left;
		cursor: pointer;
		color: var(--miso-txt-accent);
		font: 400 1rem/20px var(--miso-font-base);
		letter-spacing: -0.02em;
		transition:
			box-shadow 0.15s ease-in-out,
			background 0.15s ease-in-out;
	}

	.fsel__control:focus-visible {
		outline: none;
		background: var(--miso-cont-top);
		box-shadow: var(--miso-brd-focus);
	}

	.fsel--disabled {
		opacity: 0.6;
	}

	.fsel__control:disabled {
		cursor: default;
	}

	.fsel__icon {
		display: inline-flex;
		flex-shrink: 0;
		color: var(--miso-hlt-invert);
	}

	.fsel__value {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.fsel__chevron {
		display: inline-flex;
		color: var(--miso-txt-tertiary);
	}

	.fsel__label {
		position: absolute;
		top: 8px;
		left: 16px;
		max-width: calc(100% - 56px);
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font: 400 0.75rem/16px var(--miso-font-base);
		color: var(--miso-txt-tertiary);
		pointer-events: none;
	}

	.fsel--icon .fsel__label {
		left: 42px;
	}

	.fsel__list :global(.miso-menu) {
		max-height: min(320px, 50vh);
		overflow-y: auto;
	}
</style>
