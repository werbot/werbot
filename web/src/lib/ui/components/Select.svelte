<script lang="ts" module>
	import type { IconName } from '../icons.js';

	export interface SelectOption {
		value: string;
		label: string;
		icon?: IconName;
	}
</script>

<script lang="ts">
	import Dropdown from './Dropdown.svelte';
	import Menu from './Menu.svelte';
	import MenuItem from './MenuItem.svelte';
	import Icon from './Icon.svelte';

	interface Props {
		options: SelectOption[];
		value?: string;
		placeholder?: string;
		label?: string;
		size?: 28 | 32 | 36;
		disabled?: boolean;
		onchange?: (value: string) => void;
	}

	let {
		options,
		value = $bindable(),
		placeholder = 'Select…',
		label,
		size = 32,
		disabled = false,
		onchange
	}: Props = $props();

	let current = $derived(options.find((option) => option.value === value));

	function pick(option: SelectOption, close: () => void) {
		value = option.value;
		onchange?.(option.value);
		close();
	}
</script>

<div class="miso-select">
	{#if label}
		<span class="miso-select__label">{label}</span>
	{/if}
	<Dropdown>
		{#snippet trigger({ toggle, open })}
			<button
				type="button"
				class={['miso-select__control', `miso-select__control--s${size}`, open && 'miso-select__control--open']}
				onclick={toggle}
				{disabled}
			>
				{#if current?.icon}
					<Icon name={current.icon} size={16} />
				{/if}
				<span class={['miso-select__value', !current && 'miso-select__value--empty']}>
					{current?.label ?? placeholder}
				</span>
				<Icon name="chevron-down" size={16} />
			</button>
		{/snippet}
		{#snippet children({ close })}
			<Menu minWidth={200}>
				{#each options as option (option.value)}
					<MenuItem
						icon={option.icon}
						selected={option.value === value}
						onclick={() => pick(option, close)}
					>
						{option.label}
					</MenuItem>
				{/each}
			</Menu>
		{/snippet}
	</Dropdown>
</div>

<style>
	.miso-select {
		display: inline-flex;
		flex-direction: column;
		gap: 4px;
	}

	.miso-select__label {
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-secondary);
	}

	.miso-select__control {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		min-width: 160px;
		padding: 0 8px;
		border: 0;
		border-radius: var(--miso-radius-07);
		background: var(--miso-inp-cont);
		box-shadow: var(--miso-brd-enable);
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		cursor: pointer;
		transition: box-shadow 120ms ease;
	}

	.miso-select__control--s28 { height: 28px; }
	.miso-select__control--s32 { height: 32px; }
	.miso-select__control--s36 { height: 36px; }

	.miso-select__control:hover:not(:disabled) {
		box-shadow: var(--miso-brd-enable-hover);
	}

	.miso-select__control:focus-visible,
	.miso-select__control--open {
		outline: none;
		box-shadow: var(--miso-brd-focus);
	}

	.miso-select__control:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.miso-select__value {
		flex: 1;
		text-align: left;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
	}

	.miso-select__value--empty {
		color: var(--miso-txt-muted);
	}

	.miso-select__control > :global(.miso-icon:last-child) {
		color: var(--miso-txt-tertiary);
	}
</style>
