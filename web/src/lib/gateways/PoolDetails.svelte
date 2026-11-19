<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Banner, Button, Chip, PropertyRow } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway, GatewayInstance, JoinToken } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import InstancesTable from './InstancesTable.svelte';
	import TokenDialog from './TokenDialog.svelte';
	import TokensTable from './TokensTable.svelte';

	let {
		pool,
		ondeleteInstance,
		creating = $bindable(false),
		instanceMenu = $bindable('')
	}: { pool: Gateway; ondeleteInstance: (i: GatewayInstance) => void; creating?: boolean; instanceMenu?: string } = $props();

	let tokens = $state<JoinToken[]>([]);
	let error = $state('');

	async function refresh() {
		try {
			({ tokens } = await gateways.listJoinTokens({ gatewayId: pool.id }));
			error = '';
		} catch (e) {
			error = errorMessage(e);
		}
	}

	async function revoke(t: JoinToken) {
		try {
			await gateways.revokeJoinToken({ id: t.id });
			await refresh();
		} catch (e) {
			error = errorMessage(e);
		}
	}

	onMount(() => {
		refresh();
		const timer = setInterval(refresh, 10_000);
		return () => clearInterval(timer);
	});
</script>

{#if error}<Banner tone="alert">{error}</Banner>{/if}
<div class="details">
	<div>
		<PropertyRow label={m.gateways_col_status()}>
			<Badge tone={pool.online ? 'accent' : 'ghost'}>{pool.online ? m.gateways_online() : m.gateways_offline()}</Badge>
		</PropertyRow>
		<PropertyRow label={m.gateways_col_connectors()}>
			{#each pool.connectors as c (c)}<Chip>{c}</Chip>{:else}—{/each}
		</PropertyRow>
	</div>
	<section><InstancesTable instances={pool.instances} ondelete={ondeleteInstance} bind:menu={instanceMenu} /></section>
	<section>
		<TokensTable {tokens} onrevoke={revoke} />
		<div class="cta"><Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.gateways_token_create()}</Button></div>
	</section>
</div>
{#if creating}<TokenDialog gatewayId={pool.id} bind:open={creating} oncreated={refresh} />{/if}

<style>
	.details {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-24);
	}

	section {
		overflow-x: auto;
	}

	.cta {
		margin-top: var(--miso-sp-16);
	}

	section :global(h3) {
		margin: 0 0 var(--miso-sp-08);
	}
</style>
