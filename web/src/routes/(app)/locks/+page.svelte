<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, EmptyState, Modal, Skeleton, Table, toast, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FormField from '#lib/FormField.svelte';
	import { accessRequests, locks, members, resources, teams } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { parseDuration } from '#lib/duration.js';
	import { fmt } from '#lib/format.js';
	import type { Lock } from '#lib/gen/werbot/v1/lock_pb.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

	const types: SelectOption[] = [
		{ value: 'user', label: m.locks_type_user() },
		{ value: 'team', label: m.locks_type_team() },
		{ value: 'resource', label: m.locks_type_resource() },
		{ value: 'grant', label: m.locks_type_grant() },
		{ value: 'cli_session', label: m.locks_type_cli_session() }
	];
	const typeLabel = new Map(types.map((t) => [t.value, t.label]));

	let list = $state<Lock[]>([]);
	// target options per lock type; cli_session has none (its id is typed)
	let targets = $state<Record<string, SelectOption[]>>({});
	let loaded = $state(false);
	let failed = $state(false);
	let menuOpen = $state('');
	let removing = $state<Lock | null>(null);
	let creating = $state(false);
	let form = $state({ type: 'user', target: '', reason: '', expires: '' });
	let expiresError = $state('');

	const names = $derived(new Map(Object.values(targets).flatMap((opts) => opts.map((o) => [o.value, o.label]))));

	async function refresh() {
		const [l, mem, tm, res, gr] = await Promise.all([
			locks.listLocks({}),
			members.listMembers({}),
			teams.listTeams({}),
			resources.listResources({}),
			accessRequests.listGrants({ active: true })
		]);
		list = l.locks;
		targets = {
			user: mem.members.flatMap((x) => (x.user ? [{ value: x.user.id, label: `${x.user.name} (${x.user.login})` }] : [])),
			team: tm.teams.map((t) => ({ value: t.id, label: t.name })),
			resource: res.resources.map((r) => ({ value: r.id, label: r.name })),
			grant: gr.grants.map((g) => ({ value: g.id, label: `${g.userLogin} → ${g.resourceName}` }))
		};
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	async function create(e: SubmitEvent) {
		e.preventDefault();
		const expiresIn = form.expires.trim() ? parseDuration(form.expires) : undefined;
		expiresError = expiresIn === null ? m.common_duration_invalid() : '';
		if (expiresIn === null) return;
		try {
			await locks.createLock({ targetType: form.type, targetId: form.target.trim(), reason: form.reason, expiresIn });
			toast({ tone: 'success', text: m.locks_created() });
			creating = false;
			form = { type: 'user', target: '', reason: '', expires: '' };
		} catch (err) {
			failToast(err);
		}
		// the create response has no created_at: the list shows the stored row
		await load();
	}

	async function remove() {
		const lock = removing;
		removing = null;
		if (!lock) return;
		try {
			await locks.deleteLock({ id: lock.id });
			toast({ tone: 'success', text: m.common_deleted() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<PageHeader title={m.locks_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.locks_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="lock" title={m.locks_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.locks_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.locks_target()}</th>
				<th>{m.locks_reason()}</th>
				<th>{m.requests_expires()}</th>
				<th>{m.locks_created_by()}</th>
				<th>{m.requests_created()}</th>
				<th></th>
			{/snippet}
			{#each list as lock (lock.id)}
				<tr>
					<td>
						<CellLead icon="lock">
							<span class="target">{names.get(lock.targetId) ?? lock.targetId}</span>
							{#snippet sub()}{typeLabel.get(lock.targetType) ?? lock.targetType}{/snippet}
						</CellLead>
					</td>
					<td>{lock.reason}</td>
					<td class="muted">{lock.expiresAt ? fmt(lock.expiresAt) : m.locks_until_deleted()}</td>
					<td class="muted">{names.get(lock.createdBy) ?? lock.createdBy}</td>
					<td class="muted"><Ago ts={lock.createdAt} /></td>
					<td class="more">
						<RowMenu label={m.locks_delete()} onselect={() => (removing = lock)} bind:open={() => menuOpen === lock.id, (v) => (menuOpen = v ? lock.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<Modal bind:open={creating} title={m.locks_new()} width={480}>
	<form id="create-lock" onsubmit={create}>
		<FilledSelect label={m.locks_target_type()} options={types} bind:value={form.type} onchange={() => (form.target = '')} />
		{#if form.type === 'cli_session'}
			<FormField label={m.locks_cli_session_id()} bind:value={form.target} required />
		{:else}
			<FilledSelect label={m.locks_target()} options={targets[form.type] ?? []} bind:value={form.target} />
		{/if}
		<FormField label={m.locks_reason()} bind:value={form.reason} required maxlength={1024} />
		<FormField label={m.locks_expires()} placeholder="2h" hint={m.locks_expires_hint()} error={expiresError} bind:value={form.expires} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (creating = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="create-lock" variant="solid" disabled={!form.target.trim()}>{m.locks_new()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.locks_delete()}>
	<p>{m.locks_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.locks_delete()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.target {
		font-weight: 600;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}
</style>
