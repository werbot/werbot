<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Banner, Button, EmptyState, Label, Skeleton, Table, TextField } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import NewResourceModal from '#lib/resources/NewResourceModal.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import HealthDot from '#lib/resources/HealthDot.svelte';
	import { parseLabelFilter } from '#lib/resources/labels.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Resource[]>([]);
	let filter = $state('');
	let error = $state('');
	let creating = $state(false);
	let loaded = $state(false);
	let any = $state(false);

	async function refresh() {
		error = '';
		try {
			const labelFilter = parseLabelFilter(filter);
			({ resources: list } = await resources.listResources({ labelFilter }));
			if (!Object.keys(labelFilter).length) any = list.length > 0;
			loaded = true;
		} catch (e) {
			error = errorMessage(e);
		}
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
	{#if error}<Banner tone="alert">{error}</Banner>{/if}

	{#if !loaded}
		{#if !error}<Skeleton variant="text" lines={6} />{/if}
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
				<th>{m.resources_category()}</th>
				<th>{m.resources_labels()}</th>
				<th>{m.resources_credentials()}</th>
				<th>{m.resources_col_health()}</th>
			{/snippet}
			{#each list as r (r.id)}
				<tr>
					<td><a class="name" href={`/resources/${r.id}`}>{r.name}</a></td>
					<td>{r.type}</td>
					<td>{r.category}</td>
					<td>
						<div class="chips">
							{#each Object.entries(r.labels) as [k, v] (k)}<Label variant="soft">{k}={v}</Label>{/each}
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
</PageBody>

<style>
	.name {
		font-weight: 600;
		color: inherit;
		text-decoration: none;
	}

	.name:focus-visible {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 4px;
	}
</style>
