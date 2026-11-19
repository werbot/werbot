<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Badge, Button, Dropdown } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** number of filters currently set */
		active: number;
		children: Snippet;
	}

	let { active, children: body }: Props = $props();
</script>

<Dropdown placement="bottom-end">
	{#snippet trigger({ toggle })}
		<Button scheme="g" variant="plain" size={28} iconLeft="filter" onclick={toggle}>
			{m.common_filter()}
			{#if active > 0}<Badge tone="accent">{active}</Badge>{/if}
		</Button>
	{/snippet}
	{#snippet children()}
		<div class="filter-menu">{@render body()}</div>
	{/snippet}
</Dropdown>

<style>
	.filter-menu {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		width: 280px;
		padding: var(--miso-sp-16);
		background: var(--miso-cont-top);
		box-shadow: var(--miso-shadow-cont-minor-outline);
		border-radius: var(--miso-radius-08);
	}
</style>
