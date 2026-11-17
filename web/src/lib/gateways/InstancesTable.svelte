<script lang="ts">
	import { Badge, Button, EmptyState, Table } from '#lib/ui/index.js';
	import type { GatewayInstance } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { fmt } from '#lib/format.js';

	let { instances, ondelete }: { instances: GatewayInstance[]; ondelete: (i: GatewayInstance) => void } = $props();
</script>

<h3>{m.gateways_instances_title()}</h3>
{#if instances.length === 0}
	<EmptyState title={m.gateways_instances_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.gateways_col_hostname()}</th>
			<th>{m.gateways_col_ip()}</th>
			<th>{m.gateways_col_version()}</th>
			<th></th>
			<th>{m.gateways_col_last_seen()}</th>
			<th>{m.gateways_col_enrolled()}</th>
			<th>{m.gateways_col_cert_expires()}</th>
			<th></th>
		{/snippet}
		{#each instances as i (i.id)}
			<tr>
				<td>{i.hostname}</td>
				<td>{i.clientIp}</td>
				<td>{i.version}</td>
				<td><Badge tone={i.online ? 'accent' : 'ghost'}>{i.online ? m.gateways_online() : m.gateways_offline()}</Badge></td>
				<td>{fmt(i.lastSeenAt)}</td>
				<td>{fmt(i.createdAt)}</td>
				<td>{fmt(i.certExpiresAt)}</td>
				<td><Button scheme="w" variant="plain" onclick={() => ondelete(i)}>{m.gateways_instance_delete()}</Button></td>
			</tr>
		{/each}
	</Table>
{/if}
