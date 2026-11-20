<script lang="ts">
	import { Badge, Button, InfoBlock, Label, type BadgeTone, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FormField from '#lib/FormField.svelte';
	import { failToast } from '#lib/errors.js';
	import { policy } from '#lib/api.js';
	import type { TestCommandResponse } from '#lib/gen/werbot/v1/policy_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import ClassChips from './ClassChips.svelte';
	import { verdictOf, type Verdict } from './classes.js';

	let { users, resources }: { users: SelectOption[]; resources: SelectOption[] } = $props();

	let userId = $state('');
	let resourceId = $state('');
	let command = $state('');
	let result = $state<TestCommandResponse>();

	const verdicts: Record<Verdict, { tone: BadgeTone; text: () => string }> = {
		allowed: { tone: 'accent', text: m.policies_test_allowed },
		denied: { tone: 'alert', text: m.policies_test_denied },
		approval: { tone: 'upcoming', text: m.policies_test_needs_approval }
	};
	const verdict = $derived(result ? verdicts[verdictOf(result)] : undefined);

	async function run(e: SubmitEvent) {
		e.preventDefault();
		try {
			result = await policy.testCommand({ subject: { case: 'userId', value: userId }, resourceId, command });
		} catch (err) {
			result = undefined;
			failToast(err);
		}
	}
</script>

<section>
	<h2>{m.policies_test_title()}</h2>
	<form onsubmit={run}>
		<div class="pair">
			<FilledSelect label={m.policies_test_user()} options={users} bind:value={userId} />
			<FilledSelect label={m.policies_test_resource()} options={resources} bind:value={resourceId} />
		</div>
		<FormField label={m.policies_test_command()} bind:value={command} required maxlength={65536} />
		<div><Button size={40} type="submit" variant="solid" disabled={!userId || !resourceId}>{m.policies_test_run()}</Button></div>
	</form>
	{#if result && verdict}
		<div class="result" data-testid="test-result">
			<Badge tone={verdict.tone}>{verdict.text()}</Badge>
			<ClassChips classes={result.classes} />
			{#each result.reasons as r (r)}<Label variant="soft">{r}</Label>{/each}
		</div>
		{#if result.message}<InfoBlock>{result.message}</InfoBlock>{/if}
	{/if}
</section>

<style>
	section,
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	h2 {
		margin: 0;
		font: var(--miso-font-ui-16-semi);
	}

	.pair {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: var(--miso-sp-12);
	}

	.result {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--miso-sp-08);
	}
</style>
