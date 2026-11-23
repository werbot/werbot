<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, Dropdown, Menu, MenuItem, Tooltip } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the single danger item */
		label?: string;
		disabled?: boolean;
		onselect?: () => void;
		/** other items instead of the danger one; close shuts the menu */
		items?: Snippet<[close: () => void]>;
		open?: boolean;
	}

	let { label, disabled = false, onselect, items, open = $bindable(false) }: Props = $props();
</script>

<Dropdown placement="bottom-end" bind:open>
	{#snippet trigger({ toggle })}
		<Tooltip text={m.common_more()}>
			<Button scheme="g" variant="ghost" size={28} icon="more-horiz" aria-label={m.common_more()} onclick={toggle} />
		</Tooltip>
	{/snippet}
	{#snippet children({ close })}
		<Menu>
			{#if items}
				{@render items(close)}
			{:else}
				<MenuItem
					icon="trash"
					danger
					{disabled}
					onclick={() => {
						close();
						onselect?.();
					}}
				>
					{label}
				</MenuItem>
			{/if}
		</Menu>
	{/snippet}
</Dropdown>
