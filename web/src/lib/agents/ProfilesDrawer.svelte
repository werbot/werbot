<script lang="ts">
	import { Button, Drawer, EmptyState, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import Ago from '#lib/Ago.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { agentProfiles } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { fmt } from '#lib/format.js';
	import type { Agent } from '#lib/gen/werbot/v1/agent_pb.js';
	import type { AgentProfile } from '#lib/gen/werbot/v1/agent_profile_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the agent whose profiles to show; null closes the drawer */
		agent: Agent | null;
		onclose: () => void;
	}

	let { agent, onclose }: Props = $props();

	const shown = keepLast(() => agent);
	let list = $state<AgentProfile[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let revoking = $state<AgentProfile | null>(null);

	async function load() {
		if (!agent) return;
		try {
			({ profiles: list } = await agentProfiles.listAgentProfiles({ agentId: agent.id }));
			loaded = true;
			failed = false;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	// a drawer opened for an agent starts from an empty list and loads its profiles
	$effect(() => {
		if (!agent) return;
		list = [];
		loaded = false;
		load();
	});

	async function revoke() {
		const p = revoking;
		revoking = null;
		if (!p) return;
		try {
			await agentProfiles.revokeAgentProfile({ id: p.id });
			toast({ tone: 'success', text: m.agents_profiles_revoked() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}
</script>

<Drawer bind:open={() => !!agent, (v) => !v && onclose()} width={640} title={m.agents_profiles_title({ agent: shown.value?.name ?? '' })}>
	<p class="hint">{m.agents_profiles_hint()}</p>
	<pre><code>werbot agents profile create {shown.value?.name ?? ''} --dir &lt;path&gt;</code></pre>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={3} />
	{:else if list.length === 0}
		<EmptyState icon="key" title={m.agents_profiles_empty()} />
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.agents_profiles_label()}</th>
				<th>{m.agents_profiles_created()}</th>
				<th>{m.agents_profiles_last_used()}</th>
				<th>{m.agents_profiles_expires()}</th>
				<th></th>
			{/snippet}
			{#each list as p (p.id)}
				<tr>
					<td>{p.label || '—'}</td>
					<td class="muted"><Ago ts={p.createdAt} /></td>
					<td class="muted"><Ago ts={p.lastUsedAt} /></td>
					<td class="muted">{fmt(p.expiresAt)}</td>
					<td class="more"><Button size={28} scheme="g" variant="plain" onclick={() => (revoking = p)}>{m.agents_profiles_revoke()}</Button></td>
				</tr>
			{/each}
		</Table>
	{/if}
</Drawer>

<Modal open={revoking !== null} onclose={() => (revoking = null)} title={m.agents_profiles_revoke()}>
	<p>{m.agents_profiles_revoke_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (revoking = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={revoke}>{m.agents_profiles_revoke()}</Button>
	{/snippet}
</Modal>

<style>
	.hint {
		margin: 0 0 var(--miso-sp-08);
		color: var(--miso-txt-secondary);
	}

	pre {
		overflow-x: auto;
		user-select: all;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}
</style>
