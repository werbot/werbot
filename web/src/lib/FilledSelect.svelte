<script lang="ts">
	import { Dropdown, Icon, Menu, MenuItem, type SelectOption } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	// a longer list gets a search field on top of the menu
	const searchFrom = 8;

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
	let query = $state('');
	let current = $derived(options.find((o) => o.value === value));
	let found = $derived(query.trim() ? options.filter((o) => o.label.toLowerCase().includes(query.trim().toLowerCase())) : options);

	function pick(option: SelectOption) {
		open = false;
		if (option.value === value) return;
		value = option.value;
		onchange?.(value);
	}

	// Enter in the search picks the first match instead of submitting the form
	function onsearchkey(event: KeyboardEvent) {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		if (found[0]) pick(found[0]);
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
			<button {id} type="button" class="fsel__control" onclick={() => ((query = ''), toggle())} {disabled}>
				{#if current?.icon}<span class="fsel__icon"><Icon name={current.icon} size={20} /></span>{/if}
				<span class="fsel__value">{current?.label ?? ''}</span>
				<span class="fsel__chevron"><Icon name="chevron-down" size={16} /></span>
			</button>
		{/snippet}
		<Menu minWidth={width}>
			{#if options.length >= searchFrom}
				<!-- an opened long list is for searching: the field takes the focus -->
				<input class="fsel__search" type="text" placeholder={m.common_find()} aria-label={m.common_find()} bind:value={query} onkeydown={onsearchkey} {@attach (el) => el.focus()} />
			{/if}
			<div class="fsel__list miso-scroll">
				{#each found as option (option.value)}
					<MenuItem icon={option.icon} selected={option.value === value} onclick={() => pick(option)}>{option.label}</MenuItem>
				{:else}
					<div class="fsel__empty">{m.common_nothing_found()}</div>
				{/each}
			</div>
		</Menu>
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

	/* the search and empty state follow the miso-ui FieldTypeMenu */
	.fsel__search {
		width: 100%;
		box-sizing: border-box;
		margin: 0;
		padding: 8px 10px 8px 8px;
		border: 0;
		border-bottom: 1px solid var(--miso-line-tertiary);
		border-radius: 0;
		background: transparent;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-primary);
		outline: none;
	}

	.fsel__search::placeholder {
		color: var(--miso-txt-tertiary);
	}

	.fsel__list {
		display: flex;
		flex-direction: column;
		gap: 1px;
		max-height: min(320px, 50vh);
		overflow-x: hidden;
		overflow-y: auto;
	}

	.fsel__empty {
		padding: 12px 10px;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-tertiary);
		text-align: center;
	}
</style>
