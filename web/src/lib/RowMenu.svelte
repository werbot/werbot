<script lang="ts">
	import { Button, Dropdown, Menu, MenuItem, Tooltip } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the single danger item */
		label: string;
		disabled?: boolean;
		onselect: () => void;
		open?: boolean;
	}

	let { label, disabled = false, onselect, open = $bindable(false) }: Props = $props();
</script>

<Dropdown placement="bottom-end" bind:open>
	{#snippet trigger({ toggle })}
		<Tooltip text={m.common_more()}>
			<Button scheme="g" variant="ghost" size={28} icon="more-horiz" aria-label={m.common_more()} onclick={toggle} />
		</Tooltip>
	{/snippet}
	{#snippet children({ close })}
		<Menu>
			<MenuItem
				icon="trash"
				danger
				{disabled}
				onclick={() => {
					close();
					onselect();
				}}
			>
				{label}
			</MenuItem>
		</Menu>
	{/snippet}
</Dropdown>
