<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner, Button } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Gateway, GatewayInstance, JoinToken } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import InstancesTable from './InstancesTable.svelte';
	import TokenDialog from './TokenDialog.svelte';
	import TokensTable from './TokensTable.svelte';

	let { pool, ondeleteInstance }: { pool: Gateway; ondeleteInstance: (i: GatewayInstance) => void } = $props();

	let tokens = $state<JoinToken[]>([]);
	let error = $state('');
	let creating = $state(false);

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
<InstancesTable instances={pool.instances} ondelete={ondeleteInstance} />
<TokensTable {tokens} onrevoke={revoke} />
<p><Button variant="solid" onclick={() => (creating = true)}>{m.gateways_token_create()}</Button></p>
{#if creating}<TokenDialog gatewayId={pool.id} bind:open={creating} oncreated={refresh} />{/if}
