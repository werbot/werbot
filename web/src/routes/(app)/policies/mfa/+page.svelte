<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Drawer, EmptyState, InfoBlock, Label, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration } from '#lib/duration.js';
	import type { MFARule } from '#lib/gen/werbot/v1/policy_mfa_pb.js';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { maxAgeInvalid, fromDraft, toDraft, type MfaRuleDraft } from '#lib/mfa/ruleDraft.js';
	import RuleForm from '#lib/mfa/RuleForm.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import ScopeCells from '#lib/policies/ScopeCells.svelte';
	import { loadScopeOptions, noScope } from '#lib/policies/scopeOptions.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<MFARule[]>([]);
	let scope = $state(noScope);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<MFARule | null>(null);
	// the drawer edits `editing` ('new' for a new rule) through `draft`
	let editing = $state('');
	let draft = $state<MfaRuleDraft>(toDraft());
	let submitted = $state(false);

	const current = $derived(list.find((r) => r.id === editing));

	async function refresh() {
		const [r, s] = await Promise.all([policy.listMFARules({}), loadScopeOptions()]);
		list = r.rules;
		scope = s;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(rule?: MFARule) {
		draft = toDraft(rule);
		submitted = false;
		editing = rule?.id ?? 'new';
	}

	async function save() {
		submitted = true;
		if (maxAgeInvalid(draft.maxAge)) return;
		const input = fromDraft(draft);
		try {
			if (editing === 'new') await policy.createMFARule(input);
			else await policy.updateMFARule({ ...input, id: editing });
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
			await policy.deleteMFARule({ id: rule.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === rule.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<PageHeader title={m.mfa_rules_title()}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.mfa_rules_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else if list.length === 0}
		<EmptyState icon="shield" title={m.mfa_rules_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.mfa_rules_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.mfa_rules_name()}</th>
				<th>{m.mfa_rules_max_age()}</th>
				<th>{m.mfa_rules_strength()}</th>
				<th>{m.policies_commands_subjects()}</th>
				<th>{m.approvals_resources()}</th>
				<th></th>
			{/snippet}
			{#each list as rule (rule.id)}
				<tr>
					<td>
						<CellLead icon="shield">
							<button class="row-name" onclick={() => edit(rule)}>{rule.name}</button>
						</CellLead>
					</td>
					<td>{formatDuration(rule.maxAge)}</td>
					<td><Label variant="soft">{rule.strength === 'webauthn' ? m.mfa_rules_strength_webauthn() : m.mfa_rules_strength_any()}</Label></td>
					<ScopeCells subjects={rule.subjects} selector={rule.resourceSelector} names={scope.names} />
					<td class="more">
						<RowMenu label={m.mfa_rules_delete()} onselect={() => (removing = rule)} bind:open={() => menuOpen === rule.id, (v) => (menuOpen = v ? rule.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
	<InfoBlock>{m.mfa_rules_hint()}</InfoBlock>
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a select popup, the delete confirm or a menu is open -->
<Drawer
	bind:open={() => editing !== '', (v) => !v && !selectOpen() && !removing && !menuOpen && (editing = '')}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={current?.name ?? m.mfa_rules_new()}
>
	{#if editing}
		<RuleForm id="mfa-rule" bind:draft users={scope.users} teams={scope.teams} resources={scope.resources} {submitted} onsubmit={save} />
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.mfa_rules_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="mfa-rule" variant="solid">{current ? m.settings_save() : m.mfa_rules_create()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.mfa_rules_delete()}>
	<p>{m.mfa_rules_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.mfa_rules_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.more {
		text-align: right;
	}

	.grow {
		flex: 1;
	}
</style>
