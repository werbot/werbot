<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Badge, Button, Divider, Dropdown } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** number of filters currently set */
		active: number;
		/** selects and text fields */
		children?: Snippet;
		/** the period calendar */
		calendar: Snippet;
		/** bottom right, e.g. a reset button */
		footer?: Snippet;
	}

	let { active, children: fields, calendar, footer }: Props = $props();
</script>

<Dropdown placement="bottom-end">
	{#snippet trigger({ toggle })}
		<Button scheme="g" variant="plain" size={28} iconLeft="filter" onclick={toggle}>
			{m.common_filter()}
			{#if active > 0}<Badge tone="accent">{active}</Badge>{/if}
		</Button>
	{/snippet}
	{#snippet children()}
		<div class="filter-menu">
			<div class="filter-menu__fields">{@render fields?.()}</div>
			<Divider gap={0} />
			<span class="filter-menu__label">{m.common_period()}</span>
			{@render calendar()}
			{#if footer}<div class="filter-menu__footer">{@render footer()}</div>{/if}
		</div>
	{/snippet}
</Dropdown>

<style>
	.filter-menu {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		box-sizing: border-box;
		width: 284px;
		padding: var(--miso-sp-16);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-08);
	}

	.filter-menu__label {
		font: var(--miso-font-ui-12-med);
		color: var(--miso-txt-secondary);
	}

	/* no box of its own, so an empty one (members have no user or resource filter) leaves no gap or divider */
	.filter-menu__fields {
		display: contents;
	}

	.filter-menu__fields:empty + :global(.miso-divider) {
		display: none;
	}

	.filter-menu__footer {
		display: flex;
		justify-content: flex-end;
	}

	/* the calendar is part of the panel, not a card inside it */
	.filter-menu :global(.miso-select),
	.filter-menu :global(.miso-select__control) {
		width: 100%;
	}

	.filter-menu :global(.miso-calendar) {
		padding: 0;
		border-radius: 0;
		background: none;
		box-shadow: none;
	}
</style>
