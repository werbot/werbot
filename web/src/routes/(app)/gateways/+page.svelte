<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Badge, Banner, Button, Chip, Drawer, Dropdown, EmptyState, Menu, MenuItem, Modal, Skeleton, Table, TextField, Tooltip, toast } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import PoolDetails from '#lib/gateways/PoolDetails.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Gateway[]>([]);
	let name = $state('');
	let error = $state('');
	let createError = $state('');
	let creating = $state(false);
	let loaded = $state(false);
	let menuOpen = $state('');
	let pollError = $state('');
	let open = $state('');
	let confirm = $state<{ title: string; text: string; run: () => Promise<unknown>; precondition?: string }>();
	let confirming = $state(false);

	async function refresh() {
		try {
			({ gateways: list } = await gateways.listGateways({}));
			pollError = '';
			loaded = true;
		} catch (e) {
			pollError = errorMessage(e);
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		createError = '';
		try {
			await gateways.createGateway({ name });
			name = '';
			creating = false;
			toast({ tone: 'success', text: m.common_saved() });
			await refresh();
		} catch (err) {
			createError = errorMessage(err);
		}
	}

	function ask(title: string, text: string, run: () => Promise<unknown>, precondition?: string) {
		confirm = { title, text, run, precondition };
		confirming = true;
	}

	async function runConfirmed() {
		error = '';
		try {
			await confirm?.run();
			toast({ tone: 'success', text: m.common_saved() });
			await refresh();
		} catch (e) {
			const text = confirm?.precondition && ConnectError.from(e).code === Code.FailedPrecondition ? confirm.precondition : errorMessage(e);
			// the page banner sits behind the drawer overlay
			if (open) toast({ tone: 'alert', text });
			else error = text;
		}
		confirming = false;
	}

	const current = $derived(list.find((p) => p.id === open));

	// the open pool vanished from a refetch (deleted elsewhere): close its drawer
	$effect(() => {
		if (loaded && open && !current) open = '';
	});

	function askDeletePool(pool: Gateway) {
		ask(m.gateways_delete(), m.gateways_delete_confirm(), () => gateways.deleteGateway({ id: pool.id }), m.gateways_delete_has_resources());
	}

	onMount(() => {
		refresh();
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});
</script>

{#snippet moreMenu(pool: Gateway, slot: string)}
	<Dropdown placement="bottom-end" bind:open={() => menuOpen === slot + pool.id, (v) => (menuOpen = v ? slot + pool.id : '')}>
		{#snippet trigger({ toggle })}
			<Tooltip text={m.common_more()}>
				<Button scheme="g" variant="ghost" size={28} icon="more-horiz" aria-label={m.common_more()} onclick={toggle} />
			</Tooltip>
		{/snippet}
		{#snippet children({ close })}
			<Menu>
				<MenuItem
					icon="trash"
					danger
					onclick={() => {
						close();
						askDeletePool(pool);
					}}
				>
					{m.gateways_delete()}
				</MenuItem>
			</Menu>
		{/snippet}
	</Dropdown>
{/snippet}

<PageHeader title={m.gateways_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.gateways_create()}</Button>
	{/snippet}
</PageHeader>
{#if error || pollError}<Banner tone="alert">{error || pollError}</Banner>{/if}

{#if !loaded}
	{#if !pollError}<Skeleton variant="text" lines={6} />{/if}
{:else if list.length === 0}
	<EmptyState icon="hub" title={m.gateways_empty()}>
		{#snippet actions()}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.gateways_create()}</Button>
		{/snippet}
	</EmptyState>
{:else}
	<Table>
		{#snippet header()}
			<th>{m.gateways_col_name()}</th>
			<th>{m.gateways_col_status()}</th>
			<th>{m.gateways_col_connectors()}</th>
			<th>{m.gateways_instances_title()}</th>
			<th></th>
		{/snippet}
		{#each list as pool (pool.id)}
			<tr>
				<td><button class="name" onclick={() => (open = pool.id)}>{pool.name}</button></td>
				<td><Badge tone={pool.online ? 'accent' : 'ghost'}>{pool.online ? m.gateways_online() : m.gateways_offline()}</Badge></td>
				<td>
					<span class="chips">{#each pool.connectors as c (c)}<Chip>{c}</Chip>{/each}</span>
				</td>
				<td>{pool.instances.length}</td>
				<td class="more">{@render moreMenu(pool, 'row')}</td>
			</tr>
		{/each}
	</Table>
{/if}

<!-- Escape reaches the drawer too: ignore it while the confirm or a menu is open -->
<Drawer bind:open={() => !!current, (v) => !v && !confirming && !menuOpen && (open = '')} resizable snapPoints={[480, 720]} width={720} title={current?.name}>
	{#if current}
		<div class="actions">{@render moreMenu(current, 'drawer')}</div>
		<PoolDetails
			pool={current}
			ondeleteInstance={(i) =>
				ask(m.gateways_instance_delete(), m.gateways_instance_delete_confirm(), () => gateways.deleteGatewayInstance({ id: i.id }))}
		/>
	{/if}
</Drawer>

<Modal bind:open={creating} title={m.gateways_create()} width={480}>
	{#if createError}<Banner tone="alert">{createError}</Banner>{/if}
	<form id="pool-create" onsubmit={create}>
		<TextField label={m.gateways_name_label()} bind:value={name} required maxlength={63} />
	</form>
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => (creating = false)}>{m.common_cancel()}</Button>
		<Button type="submit" form="pool-create" variant="solid">{m.gateways_create()}</Button>
	{/snippet}
</Modal>

<Modal bind:open={confirming} title={confirm?.title}>
	{confirm?.text}
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => (confirming = false)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={runConfirmed}>{confirm?.title}</Button>
	{/snippet}
</Modal>

<style>
	.name:focus-visible {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}

	.name {
		font-weight: 600;
		background: none;
		border: 0;
		padding: 0;
		cursor: pointer;
		color: inherit;
	}

	.chips {
		display: flex;
		gap: var(--miso-sp-04);
	}

	.more,
	.actions {
		text-align: right;
	}
</style>
