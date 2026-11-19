<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Badge, Banner, Button, Chip, Drawer, EmptyState, Modal, Skeleton, Table, FilledField, toast } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import PoolDetails from '#lib/gateways/PoolDetails.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
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
	let tokenOpen = $state(false);

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
			toast({ tone: 'success', text: m.common_deleted() });
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
		if (loaded && open && !current) {
			open = '';
			tokenOpen = false;
		}
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
	<RowMenu label={m.gateways_delete()} onselect={() => askDeletePool(pool)} bind:open={() => menuOpen === slot + pool.id, (v) => (menuOpen = v ? slot + pool.id : '')} />
{/snippet}

<PageHeader title={m.gateways_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.gateways_create()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if error || pollError}<Banner tone="alert">{error || pollError}</Banner>{/if}

	{#if !loaded}
		{#if !pollError}<Skeleton variant="text" lines={6} />{/if}
	{:else if list.length === 0}
		<EmptyState icon="shield" title={m.gateways_empty()}>
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
					<td><button class="row-name" onclick={() => (open = pool.id)}>{pool.name}</button></td>
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
</PageBody>

<!-- Escape reaches the drawer too: ignore it while the confirm, the token dialog or a menu is open -->
<Drawer bind:open={() => !!current, (v) => !v && !confirming && !tokenOpen && !menuOpen && (open = '')} resizable snapPoints={[480, 720]} width={720} title={current?.name}>
	{#if current}
		<div class="actions">{@render moreMenu(current, 'drawer')}</div>
		<PoolDetails
			pool={current}
			bind:creating={tokenOpen}
			ondeleteInstance={(i) =>
				ask(m.gateways_instance_delete(), m.gateways_instance_delete_confirm(), () => gateways.deleteGatewayInstance({ id: i.id }))}
		/>
	{/if}
</Drawer>

<Modal bind:open={creating} title={m.gateways_create()} width={480}>
	{#if createError}<Banner tone="alert">{createError}</Banner>{/if}
	<form id="pool-create" onsubmit={create}>
		<FilledField label={m.gateways_name_label()} bind:value={name} required maxlength={63} />
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
	.chips {
		display: flex;
		gap: var(--miso-sp-04);
	}

	.more,
	.actions {
		text-align: right;
	}
</style>
