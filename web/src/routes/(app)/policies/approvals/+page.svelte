<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Chip, Drawer, EmptyState, Label, Modal, Skeleton, Table, toast, type SelectOption } from '#lib/ui/index.js';
	import { approvalPolicies, members, resources, teams } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { formatDuration } from '#lib/duration.js';
	import type { ApprovalPolicy } from '#lib/gen/werbot/v1/approval_policy_pb.js';
	import ApprovalPolicyForm from '#lib/requests/ApprovalPolicyForm.svelte';
	import { fromDraft, toDraft, type PolicyDraft } from '#lib/requests/policyDraft.js';
	import LabelChips from '#lib/resources/LabelChips.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { userOptions } from '#lib/members/options.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<ApprovalPolicy[]>([]);
	let users = $state<SelectOption[]>([]);
	let teamOptions = $state<SelectOption[]>([]);
	let resourceOptions = $state<SelectOption[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<ApprovalPolicy | null>(null);
	// the drawer edits `editing` ('new' for a new policy) through `draft`
	let editing = $state('');
	let draft = $state<PolicyDraft>(toDraft());
	let durationError = $state('');

	const resourceName = $derived(new Map(resourceOptions.map((o) => [o.value, o.label])));
	const current = $derived(list.find((p) => p.id === editing));

	async function refresh() {
		const [p, mem, tm, res] = await Promise.all([
			approvalPolicies.listApprovalPolicies({}),
			members.listMembers({}),
			teams.listTeams({}),
			resources.listResources({})
		]);
		list = p.approvalPolicies;
		users = userOptions(mem.members);
		teamOptions = tm.teams.map((t) => ({ value: t.id, label: t.name }));
		resourceOptions = res.resources.map((r) => ({ value: r.id, label: r.name }));
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(policy?: ApprovalPolicy) {
		draft = toDraft(policy);
		durationError = '';
		editing = policy?.id ?? 'new';
	}

	async function save() {
		const input = fromDraft(draft);
		durationError = input ? '' : m.common_duration_invalid();
		if (!input) return;
		try {
			if (editing === 'new') await approvalPolicies.createApprovalPolicy(input);
			else await approvalPolicies.updateApprovalPolicy({ ...input, id: editing });
			toast({ tone: 'success', text: m.common_saved() });
			editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	async function remove() {
		const policy = removing;
		removing = null;
		if (!policy) return;
		try {
			await approvalPolicies.deleteApprovalPolicy({ id: policy.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === policy.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<PageHeader title={m.approvals_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.approvals_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="filter_check" title={m.approvals_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.approvals_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.approvals_name()}</th>
				<th>{m.approvals_resources()}</th>
				<th>{m.approvals_mode()}</th>
				<th>{m.approvals_max_duration()}</th>
				<th></th>
			{/snippet}
			{#each list as policy (policy.id)}
				{@const ids = policy.resourceSelector?.resourceIds ?? []}
				<tr>
					<td>
						<CellLead icon="filter_check">
							<button class="row-name" onclick={() => edit(policy)}>{policy.name}</button>
							{#snippet sub()}{policy.requesters?.allUsers ? m.approvals_all_users() : ''}{/snippet}
						</CellLead>
					</td>
					<td>
						{#if ids.length > 0}
							<div class="chips">{#each ids as id (id)}<Chip size={20}>{resourceName.get(id) ?? id}</Chip>{/each}</div>
						{:else}
							<LabelChips labels={policy.resourceSelector?.labels ?? {}} />
						{/if}
					</td>
					<td><Label variant="soft">{policy.mode === 'auto' ? m.approvals_mode_auto() : m.approvals_mode_manual()}</Label></td>
					<td class="muted">{formatDuration(policy.maxDuration)}</td>
					<td class="more">
						<RowMenu label={m.approvals_delete()} onselect={() => (removing = policy)} bind:open={() => menuOpen === policy.id, (v) => (menuOpen = v ? policy.id : '')} />
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
	title={current?.name ?? m.approvals_new()}
>
	{#if editing}
		<ApprovalPolicyForm id="approval-policy" bind:draft {users} teams={teamOptions} resources={resourceOptions} {durationError} onsubmit={save} />
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.approvals_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="approval-policy" variant="solid">{current ? m.settings_save() : m.approvals_create()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.approvals_delete()}>
	<p>{m.approvals_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.approvals_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}

	.grow {
		flex: 1;
	}
</style>
