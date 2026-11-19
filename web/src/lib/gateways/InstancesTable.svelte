<script lang="ts">
	import { Badge, EmptyState, Table } from '#lib/ui/index.js';
	import type { GatewayInstance } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import { fmtShort as fmt } from '#lib/format.js';

	// id of the instance whose menu is open; the drawer guard reads it so Escape closes only the menu
	let { instances, ondelete, menu = $bindable('') }: { instances: GatewayInstance[]; ondelete: (i: GatewayInstance) => void; menu?: string } = $props();
</script>

<h3>{m.gateways_instances_title()}</h3>
{#if instances.length === 0}
	<EmptyState title={m.gateways_instances_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.gateways_col_hostname()}</th>
			<th>{m.gateways_col_ip()}</th>
			<th>{m.gateways_col_last_seen()}</th>
			<th>{m.gateways_col_cert_expires()}</th>
			<th></th>
		{/snippet}
		{#each instances as i (i.id)}
			<tr>
				<td><div class="host">{i.hostname}<Badge tone={i.online ? 'accent' : 'ghost'}>{i.online ? m.gateways_online() : m.gateways_offline()}</Badge></div></td>
				<td class="ip">{i.clientIp}<small>{i.version}</small></td>
				<td>{fmt(i.lastSeenAt)}</td>
				<td>{fmt(i.certExpiresAt)}</td>
				<td class="more"><RowMenu label={m.gateways_instance_delete()} onselect={() => ondelete(i)} bind:open={() => menu === i.id, (v) => (menu = v ? i.id : '')} /></td>
			</tr>
		{/each}
	</Table>
{/if}

<style>
	.ip {
		white-space: nowrap;
	}

	small {
		display: block;
		color: var(--miso-txt-secondary);
	}

	.host {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: var(--miso-sp-04);
	}
</style>
