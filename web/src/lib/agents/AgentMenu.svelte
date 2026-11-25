<script lang="ts">
	import RowMenu from '#lib/RowMenu.svelte';
	import { MenuItem } from '#lib/ui/index.js';
	import type { Agent } from '#lib/gen/werbot/v1/agent_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		agent: Agent;
		/** owner and admin: the kill switch */
		manager: boolean;
		open?: boolean;
		onlimits: () => void;
		onprofiles: () => void;
		onfreeze: () => void;
		onkill: (type: 'agent' | 'agents_of_user') => void;
		ondelete: () => void;
	}

	let { agent, manager, open = $bindable(false), onlimits, onprofiles, onfreeze, onkill, ondelete }: Props = $props();
</script>

<RowMenu bind:open>
	{#snippet items(close)}
		<MenuItem icon="gear" onclick={() => (close(), onlimits())}>{m.agents_action_limits()}</MenuItem>
		<MenuItem icon="key" onclick={() => (close(), onprofiles())}>{m.agents_action_profiles()}</MenuItem>
		<MenuItem icon={agent.status === 'frozen' ? 'activity' : 'hourglass'} onclick={() => (close(), onfreeze())}>
			{agent.status === 'frozen' ? m.agents_action_unfreeze() : m.agents_action_freeze()}
		</MenuItem>
		{#if manager}
			<MenuItem icon="lock" onclick={() => (close(), onkill('agent'))}>{m.agents_kill_agent()}</MenuItem>
			<MenuItem icon="lock" onclick={() => (close(), onkill('agents_of_user'))}>{m.agents_kill_sponsor()}</MenuItem>
		{/if}
		<MenuItem icon="trash" danger onclick={() => (close(), ondelete())}>{m.agents_action_delete()}</MenuItem>
	{/snippet}
</RowMenu>
