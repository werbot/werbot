<script lang="ts">
	import RowMenu from '#lib/RowMenu.svelte';
	import { MenuItem } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		id: string;
		open?: boolean;
		onterminate: () => void;
	}

	let { id, open = $bindable(false), onterminate }: Props = $props();
</script>

<RowMenu bind:open>
	{#snippet items(close)}
		<MenuItem icon="eye" href={`/sessions/${id}/live`}>{m.sessions_watch()}</MenuItem>
		<MenuItem icon="close" danger onclick={() => (close(), onterminate())}>{m.live_terminate()}</MenuItem>
	{/snippet}
</RowMenu>
