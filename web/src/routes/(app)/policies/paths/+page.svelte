<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Button, Drawer, EmptyState, Modal, Skeleton, Table, toast, type BadgeTone } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { failToast, serverText } from '#lib/errors.js';
	import type { PathRule } from '#lib/gen/werbot/v1/policy_paths_pb.js';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import PathChips from '#lib/policies/PathChips.svelte';
	import PathRuleForm from '#lib/policies/PathRuleForm.svelte';
	import ScopeCells from '#lib/policies/ScopeCells.svelte';
	import { loadScopeOptions, noScope } from '#lib/policies/scopeOptions.js';
	import { ACCESS_LABELS, EFFECT_LABELS, fromPathDraft, toPathDraft, type PathDraft } from '#lib/policies/paths.js';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<PathRule[]>([]);
	let scope = $state(noScope);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<PathRule | null>(null);
	// the drawer edits `editing` ('new' for a new rule) through `draft`
	let editing = $state('');
	let draft = $state<PathDraft>(toPathDraft());

	const current = $derived(list.find((r) => r.id === editing));

	async function refresh() {
		const [r, s] = await Promise.all([policy.listPathRules({}), loadScopeOptions()]);
		list = r.rules;
		scope = s;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(rule?: PathRule) {
		draft = toPathDraft(rule);
		editing = rule?.id ?? 'new';
	}

	async function save() {
		const input = fromPathDraft(draft);
		try {
			if (editing === 'new') await policy.createPathRule(input);
			else await policy.updatePathRule({ ...input, id: editing });
			toast({ tone: 'success', text: m.common_saved() });
			editing = '';
		} catch (e) {
			// a refused glob keeps its server text
			failToast(serverText(e));
		}
		await load();
	}

	async function remove() {
		const rule = removing;
		removing = null;
		if (!rule) return;
		try {
			await policy.deletePathRule({ id: rule.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === rule.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	const effectTones: Record<string, BadgeTone> = { only: 'accent', deny: 'alert', approval: 'upcoming' };
	const label = (labels: Record<string, () => string>, key: string) => labels[key]?.() ?? key;

	onMount(load);
</script>

<PageHeader title={m.policies_paths_title()}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_paths_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody width={960}>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={4} />
	{:else if list.length === 0}
		<EmptyState icon="folder-open" title={m.policies_paths_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_paths_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.policies_paths_name()}</th>
				<th>{m.policies_paths_effect()}</th>
				<th>{m.policies_paths_access()}</th>
				<th>{m.policies_paths_paths()}</th>
				<th>{m.policies_commands_subjects()}</th>
				<th>{m.approvals_resources()}</th>
				<th></th>
			{/snippet}
			{#each list as rule (rule.id)}
				<tr>
					<td>
						<CellLead icon="folder-open">
							<button class="row-name" onclick={() => edit(rule)}>{rule.name}</button>
							{#snippet sub()}{rule.message}{/snippet}
						</CellLead>
					</td>
					<td><Badge tone={effectTones[rule.effect] ?? 'base'}>{label(EFFECT_LABELS, rule.effect)}</Badge></td>
					<td>{label(ACCESS_LABELS, rule.access)}</td>
					<td>
						<PathChips paths={rule.paths} />
					</td>
					<ScopeCells subjects={rule.subjects} selector={rule.resourceSelector} names={scope.names} />
					<td class="more">
						<RowMenu label={m.policies_paths_delete()} onselect={() => (removing = rule)} bind:open={() => menuOpen === rule.id, (v) => (menuOpen = v ? rule.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a select popup, the delete confirm or a menu is open -->
<Drawer
	bind:open={() => editing !== '', (v) => !v && !selectOpen() && !removing && !menuOpen && (editing = '')}
	resizable
	snapPoints={[560, 720]}
	width={560}
	title={current?.name ?? m.policies_paths_new()}
>
	{#if editing}
		<PathRuleForm id="path-rule" bind:draft users={scope.users} teams={scope.teams} resources={scope.resources} onsubmit={save} />
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.policies_paths_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="path-rule" variant="solid">{current ? m.settings_save() : m.policies_paths_create()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.policies_paths_delete()}>
	<p>{m.policies_paths_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.policies_paths_delete()}</Button>
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
