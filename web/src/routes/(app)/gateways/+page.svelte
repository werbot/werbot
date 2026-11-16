<script lang="ts">
	import { onMount } from 'svelte';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Badge, Banner, Button, Chip, EmptyState, Modal, TextField, toast } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import PoolDetails from '#lib/gateways/PoolDetails.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Gateway[]>([]);
	let name = $state('');
	let error = $state('');
	let open = $state('');
	let confirm = $state<{ title: string; text: string; run: () => Promise<unknown> }>();
	let confirming = $state(false);

	async function refresh() {
		try {
			({ gateways: list } = await gateways.listGateways({}));
		} catch (e) {
			error = errorMessage(e);
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			await gateways.createGateway({ name });
			name = '';
			await refresh();
		} catch (err) {
			error = errorMessage(err);
		}
	}

	function ask(title: string, text: string, run: () => Promise<unknown>) {
		confirm = { title, text, run };
		confirming = true;
	}

	async function runConfirmed() {
		error = '';
		try {
			await confirm?.run();
			toast({ tone: 'success', text: m.common_saved() });
			await refresh();
		} catch (e) {
			error = ConnectError.from(e).code === Code.FailedPrecondition ? m.gateways_delete_has_resources() : errorMessage(e);
		}
		confirming = false;
	}

	onMount(() => {
		refresh();
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});
</script>

<h1>{m.gateways_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

<form class="create" onsubmit={create}>
	<TextField label={m.gateways_name_label()} bind:value={name} required maxlength={63} />
	<Button type="submit" variant="solid">{m.gateways_create()}</Button>
</form>

{#if list.length === 0}
	<EmptyState title={m.gateways_empty()} />
{/if}
{#each list as pool (pool.id)}
	<section>
		<header>
			<button class="name" aria-expanded={open === pool.id} onclick={() => (open = open === pool.id ? '' : pool.id)}>
				{pool.name}
			</button>
			<Badge tone={pool.online ? 'accent' : 'ghost'}>{pool.online ? m.gateways_online() : m.gateways_offline()}</Badge>
			<span class="chips">
				{#each pool.connectors as c (c)}<Chip>{c}</Chip>{/each}
			</span>
			<Button
				scheme="w"
				variant="plain"
				onclick={() => ask(m.gateways_delete(), m.gateways_delete_confirm(), () => gateways.deleteGateway({ id: pool.id }))}
			>
				{m.gateways_delete()}
			</Button>
		</header>
		{#if open === pool.id}
			<PoolDetails
				{pool}
				ondeleteInstance={(i) =>
					ask(m.gateways_instance_delete(), m.gateways_instance_delete_confirm(), () =>
						gateways.deleteGatewayInstance({ id: i.id })
					)}
			/>
		{/if}
	</section>
{/each}

<Modal bind:open={confirming} title={confirm?.title}>
	{confirm?.text}
	{#snippet footer()}
		<Button variant="plain" onclick={() => (confirming = false)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={runConfirmed}>{confirm?.title}</Button>
	{/snippet}
</Modal>

<style>
	.create {
		display: flex;
		align-items: flex-end;
		gap: 12px;
		margin-bottom: 16px;
	}

	section {
		margin-bottom: 16px;
		padding: 8px 12px;
		border: 1px solid var(--miso-line-tertiary);
		border-radius: 8px;
	}

	header {
		display: flex;
		align-items: center;
		gap: 12px;
	}

	.name {
		font-weight: 600;
		background: none;
		border: 0;
		cursor: pointer;
		color: inherit;
	}

	.chips {
		display: flex;
		flex: 1;
		gap: 4px;
	}
</style>
