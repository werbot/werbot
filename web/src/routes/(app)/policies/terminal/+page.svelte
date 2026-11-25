<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Button, Drawer, EmptyState, Modal, Skeleton, Table, toast, type BadgeTone } from '#lib/ui/index.js';
	import ToneSegmented from '#lib/ToneSegmented.svelte';
	import { policy } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { TerminalRule } from '#lib/gen/werbot/v1/policy_paths_pb.js';
	import CellLead from '#lib/CellLead.svelte';
	import FormField from '#lib/FormField.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import ScopeCells from '#lib/policies/ScopeCells.svelte';
	import { loadScopeOptions, noScope } from '#lib/policies/scopeOptions.js';
	import SubjectsTarget from '#lib/policies/SubjectsTarget.svelte';
	import { fromSelectorDraft, fromSubjectsDraft, toSelectorDraft, toSubjectsDraft, type SelectorDraft, type SubjectsDraft } from '#lib/resources/selectorDraft.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	type Draft = SelectorDraft & { name: string; subjects: SubjectsDraft; mode: string };
	const emptyDraft = (): Draft => ({ name: '', mode: 'console', subjects: toSubjectsDraft(), ...toSelectorDraft() });

	let list = $state<TerminalRule[]>([]);
	let scope = $state(noScope);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<TerminalRule | null>(null);
	let creating = $state(false);
	let draft = $state<Draft>(emptyDraft());


	async function refresh() {
		const [r, s] = await Promise.all([policy.listTerminalRules({}), loadScopeOptions()]);
		list = r.rules;
		scope = s;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function create() {
		draft = emptyDraft();
		creating = true;
	}

	async function save() {
		try {
			await policy.createTerminalRule({
				name: draft.name,
				mode: draft.mode,
				subjects: fromSubjectsDraft(draft.subjects),
				resourceSelector: fromSelectorDraft(draft)
			});
			toast({ tone: 'success', text: m.common_saved() });
			creating = false;
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	async function remove() {
		const rule = removing;
		removing = null;
		if (!rule) return;
		try {
			await policy.deleteTerminalRule({ id: rule.id });
			toast({ tone: 'success', text: m.common_deleted() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	const modes: Record<string, { tone: BadgeTone; text: () => string }> = {
		console: { tone: 'accent', text: m.policies_terminal_mode_console },
		deny: { tone: 'alert', text: m.policies_terminal_mode_deny }
	};

	onMount(load);
</script>

<PageHeader title={m.policies_terminal_title()}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={create}>{m.policies_terminal_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else if list.length === 0}
		<EmptyState icon="keyboard" title={m.policies_terminal_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={create}>{m.policies_terminal_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.policies_terminal_name()}</th>
				<th>{m.policies_terminal_mode()}</th>
				<th>{m.policies_commands_subjects()}</th>
				<th>{m.approvals_resources()}</th>
				<th></th>
			{/snippet}
			{#each list as rule (rule.id)}
				{@const mode = modes[rule.mode]}
				<tr>
					<td><CellLead icon="keyboard">{rule.name}</CellLead></td>
					<td><Badge tone={mode?.tone ?? 'base'}>{mode?.text() ?? rule.mode}</Badge></td>
					<ScopeCells subjects={rule.subjects} selector={rule.resourceSelector} names={scope.names} />
					<td class="more">
						<RowMenu label={m.policies_terminal_delete()} onselect={() => (removing = rule)} bind:open={() => menuOpen === rule.id, (v) => (menuOpen = v ? rule.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a select popup, the delete confirm or a menu is open -->
<Drawer
	bind:open={() => creating, (v) => !v && !selectOpen() && !removing && !menuOpen && (creating = false)}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={m.policies_terminal_new()}
>
	{#if creating}
		<form
			id="terminal-rule"
			onsubmit={(e) => {
				e.preventDefault();
				save();
			}}
		>
			<FormField label={m.policies_terminal_name()} bind:value={draft.name} required maxlength={63} />
			<SubjectsTarget bind:draft users={scope.users} teams={scope.teams} resources={scope.resources} />
			<fieldset>
				<legend>{m.policies_terminal_mode()}</legend>
				<div>
					<ToneSegmented
						size={32}
						options={[
							{ value: 'console', label: m.policies_terminal_mode_console(), tone: 'allow' },
							{ value: 'deny', label: m.policies_terminal_mode_deny(), tone: 'deny' }
						]}
						bind:value={draft.mode}
					/>
				</div>
				<p class="hint">{draft.mode === 'console' ? m.policies_terminal_console_hint() : m.policies_terminal_deny_hint()}</p>
			</fieldset>
		</form>
	{/if}
	{#snippet footer()}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (creating = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="terminal-rule" variant="solid">{m.policies_terminal_create()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.policies_terminal_delete()}>
	<p>{m.policies_terminal_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.policies_terminal_delete()}</Button>
	{/snippet}
</Modal>

<style>
	form,
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	form {
		gap: var(--miso-sp-24);
	}

	fieldset {
		margin: 0;
		padding: 0;
		border: 0;
		min-width: 0;
	}

	legend {
		padding: 0;
		font: var(--miso-font-ui-13-semi);
		color: var(--miso-txt-primary);
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}

	.more {
		text-align: right;
	}

	.grow {
		flex: 1;
	}
</style>
