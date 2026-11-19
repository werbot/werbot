<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Button, EmptyState, Label, Skeleton, Table, TextField } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import NewResourceModal from '#lib/resources/NewResourceModal.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import HealthDot from '#lib/resources/HealthDot.svelte';
	import LabelChips from '#lib/resources/LabelChips.svelte';
	import { address } from '#lib/resources/address.js';
	import { categoryLabel, typeIcon } from '#lib/resources/icons.js';
	import { parseLabelFilter } from '#lib/resources/labels.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Resource[]>([]);
	let filter = $state('');
	let failed = $state(false);
	let creating = $state(false);
	let loaded = $state(false);
	let any = $state(false);

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	async function refresh() {
		try {
			const labelFilter = parseLabelFilter(filter);
			({ resources: list } = await resources.listResources({ labelFilter }));
			if (!Object.keys(labelFilter).length) any = list.length > 0;
			loaded = true;
			failed = false;
		} catch (e) {
			failed = true;
			failToast(e);
		}
	}

	function resetFilter() {
		filter = '';
		refresh();
	}

	onMount(refresh);
</script>

<PageHeader title={m.resources_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				refresh();
			}}
		>
			<TextField size={28} aria-label={m.resources_label_filter()} placeholder="env=prod,team=a" bind:value={filter} />
		</form>
		{#if any}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.resources_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<NewResourceModal bind:open={creating} />
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={refresh} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0 && any}
		<EmptyState icon="desktop" title={m.resources_no_match()}>
			{#snippet actions()}
				<Button scheme="g" variant="plain" size={28} iconLeft="close" onclick={resetFilter}>{m.common_reset()}</Button>
			{/snippet}
		</EmptyState>
	{:else if list.length === 0}
		<EmptyState icon="desktop" title={m.resources_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.resources_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.resources_name()}</th>
				<th>{m.resources_type()}</th>
				<th>{m.resources_labels()}</th>
				<th>{m.resources_credentials()}</th>
				<th>{m.resources_col_health()}</th>
			{/snippet}
			{#each list as r (r.id)}
				<tr>
					<td>
						<CellLead icon={typeIcon(r.type)}>
							<a class="row-name" href={`/resources/${r.id}`}>{r.name}</a>
							{#snippet sub()}{address(r.configJson) || '—'}{/snippet}
						</CellLead>
					</td>
					<td><span class="type"><Label variant="soft">{r.type}</Label>{categoryLabel(r.category)}</span></td>
					<td><LabelChips labels={r.labels} /></td>
					<td>
						<Badge tone={r.credentialKind ? 'accent' : 'ghost'}>{r.credentialKind || m.resources_cred_none()}</Badge>
					</td>
					<td><span class="health"><HealthDot health={r.health} /><Ago ts={r.health?.checkedAt} /></span></td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<style>
	.type,
	.health {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
		color: var(--miso-txt-tertiary);
		white-space: nowrap;
	}
</style>
