<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Button, EmptyState, Label, MenuItem, Modal, Skeleton, Table, toast } from '#lib/ui/index.js';
	import { presets } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { ResourcePreset } from '#lib/gen/werbot/v1/preset_pb.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import RowMenu from '#lib/RowMenu.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import PresetDrawer from '#lib/resources/PresetDrawer.svelte';
	import { formatPorts } from '#lib/resources/forms/ports.js';
	import { categoryLabel, resourceTypeIcon } from '#lib/resources/icons.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<ResourcePreset[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let editing = $state<ResourcePreset>();
	let drawer = $state(false);
	let menuOpen = $state('');
	let removing = $state<ResourcePreset>();

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	async function load() {
		try {
			({ presets: list } = await presets.listResourcePresets({}));
			failed = false;
			loaded = true;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	function openDrawer(p?: ResourcePreset) {
		editing = p;
		drawer = true;
	}

	async function remove() {
		const p = removing;
		removing = undefined;
		if (!p) return;
		try {
			await presets.deleteResourcePreset({ id: p.id });
			toast({ tone: 'success', text: m.common_deleted() });
			await load();
		} catch (e) {
			failToast(ConnectError.from(e).code === Code.FailedPrecondition ? m.presets_delete_in_use() : e);
		}
	}

	onMount(load);
</script>

<PageHeader title={m.presets_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => openDrawer()}>{m.presets_create()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="storage" title={m.presets_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => openDrawer()}>{m.presets_create()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.presets_col_name()}</th>
				<th>{m.presets_col_base()}</th>
				<th>{m.presets_col_ports()}</th>
				<th>{m.presets_col_category()}</th>
				<th></th>
			{/snippet}
			{#each list as p (p.id)}
				<tr>
					<td>
						<CellLead icon={resourceTypeIcon(p)}>
							<button class="row-name" onclick={() => openDrawer(p)}>{p.displayName}</button>
							{#snippet sub()}{p.name}{/snippet}
						</CellLead>
					</td>
					<td><Label variant="soft">{p.baseType}</Label></td>
					<td class="muted">{formatPorts(p.defaultPorts)}</td>
					<td class="muted">{categoryLabel(p.category)}</td>
					<td class="more">
						<RowMenu bind:open={() => menuOpen === p.id, (v) => (menuOpen = v ? p.id : '')}>
							{#snippet items(close)}
								<MenuItem icon="edit" onclick={() => { close(); openDrawer(p); }}>{m.presets_edit()}</MenuItem>
								<MenuItem icon="trash" danger onclick={() => { close(); removing = p; }}>{m.presets_delete()}</MenuItem>
							{/snippet}
						</RowMenu>
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<PresetDrawer bind:open={drawer} preset={editing} onsaved={load} />

<Modal bind:open={() => !!removing, (v) => !v && (removing = undefined)} title={m.presets_delete()}>
	{m.presets_delete_confirm()}
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = undefined)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.presets_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.muted {
		color: var(--miso-txt-secondary);
	}

	.more {
		text-align: right;
	}
</style>
