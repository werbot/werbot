<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { Badge, Banner, Button, Chip, EmptyState, Table, TextField } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { parseLabelFilter } from '#lib/resources/labels.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Resource[]>([]);
	let filter = $state('');
	let error = $state('');

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

<h1>{m.resources_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

<div class="bar">
	<form
		onsubmit={(e) => {
			e.preventDefault();
			refresh();
		}}
	>
		<TextField label={m.resources_label_filter()} placeholder="env=prod,team=a" bind:value={filter} />
	</form>
	<Button variant="solid" onclick={() => goto('/resources/new')}>{m.resources_new()}</Button>
</div>

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
			</tr>
		{/each}
	</Table>
{/if}

<style>
	.bar {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 12px;
		margin-bottom: 16px;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
</style>
