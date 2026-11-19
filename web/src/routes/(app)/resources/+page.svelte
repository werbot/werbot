<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Banner, Button, Chip, EmptyState, Table, TextField } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import NewResourceModal from '#lib/resources/NewResourceModal.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import HealthDot from '#lib/resources/HealthDot.svelte';
	import { parseLabelFilter } from '#lib/resources/labels.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Resource[]>([]);
	let filter = $state('');
	let error = $state('');
	let creating = $state(false);

	async function refresh() {
		error = '';
		try {
			({ resources: list } = await resources.listResources({ labelFilter: parseLabelFilter(filter) }));
		} catch (e) {
			error = errorMessage(e);
		}
	}

	onMount(refresh);
</script>

<PageHeader title={m.resources_title()} count={list.length}>
	{#snippet toolbar()}
		<form
			onsubmit={(e) => {
				e.preventDefault();
				refresh();
			}}
		>
			<TextField size={28} aria-label={m.resources_label_filter()} placeholder="env=prod,team=a" bind:value={filter} />
		</form>
		<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.resources_new()}</Button>
	{/snippet}
</PageHeader>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<NewResourceModal bind:open={creating} />

{#if list.length === 0}
	<EmptyState title={m.resources_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.resources_name()}</th>
			<th>{m.resources_type()}</th>
			<th>{m.resources_category()}</th>
			<th>{m.resources_labels()}</th>
			<th>{m.resources_credentials()}</th>
			<th>{m.resources_col_health()}</th>
		{/snippet}
		{#each list as r (r.id)}
			<tr>
				<td><a href={`/resources/${r.id}`}>{r.name}</a></td>
				<td>{r.type}</td>
				<td>{r.category}</td>
				<td>
					<div class="chips">
						{#each Object.entries(r.labels) as [k, v] (k)}<Chip>{k}={v}</Chip>{/each}
					</div>
				</td>
				<td>
					<Badge tone={r.credentialKind ? 'accent' : 'ghost'}>{r.credentialKind || m.resources_cred_none()}</Badge>
				</td>
				<td><HealthDot health={r.health} /></td>
			</tr>
		{/each}
	</Table>
{/if}

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
</style>
