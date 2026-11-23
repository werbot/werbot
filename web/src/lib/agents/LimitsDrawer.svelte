<script lang="ts">
	import { Button, Drawer, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import FieldRow from '#lib/FieldRow.svelte';
	import { agents } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration, parseDuration } from '#lib/duration.js';
	import type { Agent } from '#lib/gen/werbot/v1/agent_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the agent to edit; null closes the drawer */
		agent: Agent | null;
		onclose: () => void;
		onsaved: () => void;
	}

	let { agent, onclose, onsaved }: Props = $props();

	const shown = keepLast(() => agent);
	let maxActions = $state('');
	let maxDuration = $state('');
	let maxConcurrent = $state('');
	let durationError = $state('');

	// a new agent: its limits fill the form; 0 and unset show as empty (the default)
	$effect(() => {
		maxActions = agent?.limits?.maxActions ? String(agent.limits.maxActions) : '';
		maxDuration = formatDuration(agent?.limits?.maxDuration);
		maxConcurrent = agent?.limits?.maxConcurrent ? String(agent.limits.maxConcurrent) : '';
		durationError = '';
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		const duration = maxDuration.trim() ? parseDuration(maxDuration) : undefined;
		durationError = duration === null ? m.common_duration_invalid() : '';
		if (!agent || duration === null) return;
		try {
			await agents.updateAgent({
				id: agent.id,
				limits: { maxActions: Number(maxActions) || 0, maxDuration: duration, maxConcurrent: Number(maxConcurrent) || 0 }
			});
			toast({ tone: 'success', text: m.agents_limits_saved() });
			onsaved();
			onclose();
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Drawer bind:open={() => !!agent, (v) => !v && onclose()} width={480} title={`${m.agents_action_limits()}: ${shown.value?.name ?? ''}`}>
	<form id="agent-limits" onsubmit={save}>
		<FormField label={m.agents_limits_max_actions()} type="number" min="0" max="10000" placeholder={m.agents_limits_default()} bind:value={maxActions} />
		<FieldRow even>
			<FormField label={m.agents_limits_max_duration()} placeholder={m.agents_limits_default()} error={durationError} bind:value={maxDuration} />
			<FormField label={m.agents_limits_max_concurrent()} type="number" min="0" max="32" placeholder={m.agents_limits_default()} bind:value={maxConcurrent} />
		</FieldRow>
		<p class="hint">{m.agents_limits_hint()}</p>
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={onclose}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="agent-limits" variant="solid">{m.settings_save()}</Button>
	{/snippet}
</Drawer>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.hint {
		margin: 0;
		color: var(--miso-txt-secondary);
		font: var(--miso-font-ui-12-reg);
	}
</style>
