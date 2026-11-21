<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, Drawer, EmptyState, Modal, Skeleton, Table, Toggle, toast } from '#lib/ui/index.js';
	import { policy } from '#lib/api.js';
	import { failToast, serverText } from '#lib/errors.js';
	import type { CustomPolicy } from '#lib/gen/werbot/v1/policy_pb.js';
	import FormField from '#lib/FormField.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import Ago from '#lib/Ago.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<CustomPolicy[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let removing = $state<CustomPolicy | null>(null);
	// the drawer edits `editing` ('new' for a new policy); the text stays after a failed save
	let editing = $state('');
	let name = $state('');
	let cedar = $state('');
	let enabled = $state(true);

	const current = $derived(list.find((p) => p.id === editing));

	async function refresh() {
		list = (await policy.listCustomPolicies({})).policies;
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	function edit(p?: CustomPolicy) {
		name = p?.name ?? '';
		cedar = p?.cedar ?? '';
		enabled = p?.enabled ?? true;
		editing = p?.id ?? 'new';
	}

	async function save() {
		try {
			if (editing === 'new') await policy.createCustomPolicy({ name, cedar, enabled });
			else await policy.updateCustomPolicy({ id: editing, name, cedar, enabled });
			toast({ tone: 'success', text: m.policies_custom_saved() });
			editing = '';
		} catch (e) {
			failToast(serverText(e));
		}
		await load();
	}

	async function toggle(p: CustomPolicy, on: boolean) {
		try {
			await policy.updateCustomPolicy({ id: p.id, name: p.name, cedar: p.cedar, enabled: on });
			toast({ tone: 'success', text: m.policies_custom_saved() });
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	async function remove() {
		const p = removing;
		removing = null;
		if (!p) return;
		try {
			await policy.deleteCustomPolicy({ id: p.id });
			toast({ tone: 'success', text: m.common_deleted() });
			if (editing === p.id) editing = '';
		} catch (e) {
			failToast(e);
		}
		await load();
	}

	onMount(load);
</script>

<PageHeader title={m.policies_custom_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_custom_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="editor/code" title={m.policies_custom_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => edit()}>{m.policies_custom_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.policies_custom_name()}</th>
				<th>{m.policies_custom_enabled()}</th>
				<th>{m.policies_custom_updated()}</th>
			{/snippet}
			{#each list as p (p.id)}
				<tr>
					<td>
						<CellLead icon="editor/code"><button class="row-name" onclick={() => edit(p)}>{p.name}</button></CellLead>
					</td>
					<td><Toggle checked={p.enabled} aria-label={`${m.policies_custom_enabled()}: ${p.name}`} onchange={(e) => toggle(p, e.currentTarget.checked)} /></td>
					<td class="muted"><Ago ts={p.updatedAt} /></td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<Drawer bind:open={() => editing !== '', (v) => !v && !removing && (editing = '')} resizable snapPoints={[640, 880]} width={640} title={current?.name ?? m.policies_custom_new()}>
	{#if editing}
		<form
			id="custom-policy"
			onsubmit={(e) => {
				e.preventDefault();
				save();
			}}
		>
			<FormField label={m.policies_custom_name()} bind:value={name} required maxlength={63} />
			<FormField multiline label={m.policies_custom_cedar()} bind:value={cedar} rows={16} required spellcheck="false" autocomplete="off" style="font-family: var(--miso-font-mono)" />
			<Toggle bind:checked={enabled}>{m.policies_custom_enabled()}</Toggle>
		</form>
	{/if}
	{#snippet footer()}
		{#if current}
			<Button size={40} scheme="w" variant="solid" iconLeft="trash" onclick={() => (removing = current ?? null)}>{m.policies_custom_delete()}</Button>
		{/if}
		<span class="grow"></span>
		<Button size={40} scheme="g" variant="plain" onclick={() => (editing = '')}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="custom-policy" variant="solid">{m.settings_save()}</Button>
	{/snippet}
</Drawer>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.policies_custom_delete()}>
	<p>{m.policies_custom_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.policies_custom_delete()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-16);
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.grow {
		flex: 1;
	}
</style>
