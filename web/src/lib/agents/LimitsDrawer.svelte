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
	let maxRows = $state('');
	let maxResultBytes = $state('');
	let statementTimeout = $state('');
	let maxQueries = $state('');
	let timeoutError = $state('');

	// 0 and unset show as empty (the default)
	const shownNumber = (n?: number | bigint) => (n ? String(n) : '');
	// empty is unset; null is not a duration
	const optionalDuration = (text: string) => (text.trim() ? parseDuration(text) : undefined);

	// a new agent: its limits fill the form
	$effect(() => {
		const l = agent?.limits;
		maxActions = shownNumber(l?.maxActions);
		maxDuration = formatDuration(l?.maxDuration);
		maxConcurrent = shownNumber(l?.maxConcurrent);
		maxRows = shownNumber(l?.maxRows);
		maxResultBytes = shownNumber(l?.maxResultBytes);
		statementTimeout = formatDuration(l?.statementTimeout);
		maxQueries = shownNumber(l?.maxQueries);
		durationError = timeoutError = '';
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		const [duration, timeout] = [optionalDuration(maxDuration), optionalDuration(statementTimeout)];
		durationError = duration === null ? m.common_duration_invalid() : '';
		timeoutError = timeout === null ? m.common_duration_invalid() : '';
		if (!agent || duration === null || timeout === null) return;
		try {
			await agents.updateAgent({
				id: agent.id,
				limits: {
					maxActions: Number(maxActions) || 0,
					maxDuration: duration,
					maxConcurrent: Number(maxConcurrent) || 0,
					maxRows: Number(maxRows) || 0,
					maxResultBytes: BigInt(Number(maxResultBytes) || 0),
					statementTimeout: timeout,
					maxQueries: Number(maxQueries) || 0
				}
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
		<FieldRow even>
			<FormField label={m.agents_limits_max_rows()} type="number" min="0" max="100000" placeholder={m.agents_limits_default()} bind:value={maxRows} />
			<FormField label={m.agents_limits_max_queries()} type="number" min="0" max="100000" placeholder={m.agents_limits_default()} bind:value={maxQueries} />
		</FieldRow>
		<FieldRow even>
			<FormField label={m.agents_limits_max_result_bytes()} type="number" min="0" max="67108864" placeholder={m.agents_limits_default()} bind:value={maxResultBytes} />
			<FormField label={m.agents_limits_statement_timeout()} placeholder={m.agents_limits_default()} error={timeoutError} bind:value={statementTimeout} />
		</FieldRow>
		<p class="hint">{m.agents_limits_db_hint()}</p>
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
