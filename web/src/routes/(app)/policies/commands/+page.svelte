<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Drawer, EmptyState, InfoBlock, Label, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { CommandRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import ClassRules from '#lib/policies/ClassRules.svelte';
	import CommandRuleForm from '#lib/policies/CommandRuleForm.svelte';
	import TestCommand from '#lib/policies/TestCommand.svelte';
	import { allowlistDraft, fromDraft, presetDraft, toDraft, type RuleDraft } from '#lib/policies/ruleDraft.js';
	import ScopeCells from '#lib/policies/ScopeCells.svelte';
	import { loadScopeOptions, noScope } from '#lib/policies/scopeOptions.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<CommandRule[]>([]);
	let scope = $state(noScope);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<CommandRule | null>(null);
	// the drawer edits `editing` ('new' for a new rule) through `draft`
	let editing = $state('');
	let draft = $state<RuleDraft>(toDraft());
	let classesError = $state(false);

	const current = $derived(list.find((r) => r.id === editing));

	async function refresh() {
		const [r, s] = await Promise.all([policy.listCommandRules({}), loadScopeOptions()]);
		list = r.rules;
		scope = s;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(rule?: CommandRule, from?: RuleDraft) {
		draft = from ?? toDraft(rule);
		classesError = false;
		editing = rule?.id ?? 'new';
	}

	async function save() {
		classesError = draft.classes.length === 0;
		if (classesError) return;
		const input = fromDraft(draft);
		try {
			if (editing === 'new') await policy.createCommandRule(input);
			else await policy.updateCommandRule({ ...input, id: editing });
			toast({ tone: 'success', text: m.common_saved() });
			editing = '';
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
			await policy.deleteCommandRule({ id: rule.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === rule.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	const effects: Record<string, () => string> = {
		allow: m.policies_commands_effect_allow,
		deny: m.policies_commands_effect_deny,
		approval: m.policies_commands_effect_approval
	};

	onMount(load);
</script>

<PageHeader title={m.policies_commands_title()}>
	{#snippet toolbar()}
		<Button variant="plain" size={40} onclick={() => edit(undefined, allowlistDraft(m.policies_commands_preset_allowlist_message()))}>{m.policies_commands_preset_allowlist()}</Button>
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_commands_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody width={960}>
	<section>
		<h2>{m.policies_commands_rules()}</h2>
		{#if failed}
			<LoadFailed onretry={load} />
		{:else if !loaded}
			<Skeleton variant="text" lines={4} />
		{:else if list.length === 0}
			<EmptyState icon="editor/checklist" title={m.policies_commands_empty()}>
				{#snippet actions()}
					<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_commands_new()}</Button>
					<Button variant="plain" size={40} onclick={() => edit(undefined, presetDraft())}>{m.policies_commands_preset_opaque()}</Button>
				{/snippet}
			</EmptyState>
		{:else}
			<Table>
				{#snippet header()}
					<th>{m.policies_commands_name()}</th>
					<th>{m.policies_commands_effect()}</th>
					<th>{m.policies_commands_classes()}</th>
					<th>{m.policies_commands_subjects()}</th>
					<th>{m.approvals_resources()}</th>
					<th></th>
				{/snippet}
				{#each list as rule (rule.id)}
					<tr>
						<td>
							<CellLead icon="editor/checklist">
								<button class="row-name" onclick={() => edit(rule)}>{rule.name}</button>
								{#snippet sub()}{rule.message}{/snippet}
							</CellLead>
						</td>
						<td><Label variant="soft">{(effects[rule.effect] ?? (() => rule.effect))()}</Label></td>
						<td><ClassChips classes={rule.classes} /></td>
						<ScopeCells subjects={rule.subjects} selector={rule.resourceSelector} names={scope.names} />
						<td class="more">
							<RowMenu label={m.policies_commands_delete()} onselect={() => (removing = rule)} bind:open={() => menuOpen === rule.id, (v) => (menuOpen = v ? rule.id : '')} />
						</td>
					</tr>
				{/each}
			</Table>
		{/if}
	</section>
	<InfoBlock>{m.policies_commands_preset_allowlist_hint()}</InfoBlock>
	<ClassRules />
	{#if loaded && !failed}<TestCommand users={scope.users} resources={scope.resources} rules={list} />{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a select popup, the delete confirm or a menu is open -->
<Drawer
	bind:open={() => editing !== '', (v) => !v && !selectOpen() && !removing && !menuOpen && (editing = '')}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={current?.name ?? m.policies_commands_new()}
>
	{#if editing}
		<CommandRuleForm id="command-rule" bind:draft users={scope.users} teams={scope.teams} resources={scope.resources} {classesError} onsubmit={save} />
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.policies_commands_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="command-rule" variant="solid">{current ? m.settings_save() : m.policies_commands_create()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.policies_commands_delete()}>
	<p>{m.policies_commands_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.policies_commands_delete()}</Button>
	{/snippet}
</Modal>

<style>
	section {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	h2 {
		margin: 0;
		font: var(--miso-font-ui-16-semi);
	}

	.more {
		text-align: right;
	}

	.grow {
		flex: 1;
	}
</style>
