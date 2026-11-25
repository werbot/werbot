<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Skeleton, Table, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { failToast } from '#lib/errors.js';
	import { policy } from '#lib/api.js';
	import type { ClassRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import ClassChips from './ClassChips.svelte';
	import ClassPicker from './ClassPicker.svelte';

	let list = $state<ClassRule[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let pattern = $state('');
	let classes = $state<string[]>([]);

	const load = () =>
		policy.listClassRules({}).then(
			(r) => ((list = r.rules), (failed = false), (loaded = true)),
			(e) => ((failed = true), failToast(e))
		);

	async function add(e: SubmitEvent) {
		e.preventDefault();
		try {
			await policy.createClassRule({ pattern: pattern.trim(), classes });
			toast({ tone: 'success', text: m.common_saved() });
			pattern = '';
			classes = [];
		} catch (err) {
			failToast(err);
		}
		await load();
	}

	async function remove(id: string) {
		try {
			await policy.deleteClassRule({ id });
			toast({ tone: 'success', text: m.common_deleted() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<section>
	<h2>{m.policies_class_rules_title()}</h2>
	<p class="hint">{m.policies_class_rules_hint()}</p>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={2} />
	{:else if list.length === 0}
		<p class="hint">{m.policies_class_rules_empty()}</p>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.policies_class_rules_pattern()}</th>
				<th>{m.policies_commands_classes()}</th>
				<th></th>
			{/snippet}
			{#each list as r (r.id)}
				<tr>
					<td class="mono">{r.pattern}</td>
					<td><ClassChips classes={r.classes} /></td>
					<td class="more"><Button size={28} scheme="w" variant="plain" iconLeft="trash" onclick={() => remove(r.id)}>{m.policies_class_rules_delete()}</Button></td>
				</tr>
			{/each}
		</Table>
	{/if}
	<form onsubmit={add}>
		<FormField label={m.policies_class_rules_pattern()} placeholder="./deploy.sh" bind:value={pattern} required maxlength={255} />
		<ClassPicker bind:selected={classes} />
		<div><Button size={40} type="submit" variant="solid" iconLeft="plus" disabled={classes.length === 0}>{m.policies_class_rules_add()}</Button></div>
	</form>
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

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.mono {
		font-family: var(--miso-font-mono, monospace);
	}

	.more {
		text-align: right;
	}
</style>
