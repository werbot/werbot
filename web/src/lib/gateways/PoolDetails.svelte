<script lang="ts">
	import { onMount } from 'svelte';
	import { Badge, Button, Chip, PropertyRow } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { Gateway, GatewayInstance, JoinToken } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import InstancesTable from './InstancesTable.svelte';
	import TokenDialog from './TokenDialog.svelte';
	import TokensTable from './TokensTable.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';

	let {
		pool,
		ondeleteInstance,
		creating = $bindable(false),
		instanceMenu = $bindable('')
	}: { pool: Gateway; ondeleteInstance: (i: GatewayInstance) => void; creating?: boolean; instanceMenu?: string } = $props();

	let tokens = $state<JoinToken[]>([]);
	let failed = $state(false);

	// polled: a failure toasts once, not on every tick; a manual Retry always toasts
	async function refresh(manual = false) {
		try {
			({ tokens } = await gateways.listJoinTokens({ gatewayId: pool.id }));
			failed = false;
		} catch (e) {
			if (!failed || manual) failToast(e);
			failed = true;
		}
	}

	async function revoke(t: JoinToken) {
		try {
			await gateways.revokeJoinToken({ id: t.id });
			await refresh();
		} catch (e) {
			failToast(e);
		}
	}

	onMount(() => {
		refresh();
		const timer = setInterval(() => refresh(), 10_000);
		return () => clearInterval(timer);
	});
</script>

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
		{#if failed}<LoadFailed onretry={() => refresh(true)} />{:else}<TokensTable {tokens} onrevoke={revoke} />{/if}
		<div class="cta"><Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.gateways_token_create()}</Button></div>
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
