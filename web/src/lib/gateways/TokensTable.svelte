<script lang="ts">
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Badge, Button, EmptyState, Table } from '#lib/ui/index.js';
	import type { JoinToken } from '#lib/gen/werbot/v1/gateway_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import Ago from '#lib/Ago.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import { fmt } from '#lib/format.js';

	let { tokens, onrevoke }: { tokens: JoinToken[]; onrevoke: (t: JoinToken) => void } = $props();

	const expired = (t: JoinToken) => !!t.expiresAt && timestampDate(t.expiresAt) < new Date();
</script>

<h3>{m.gateways_tokens_title()}</h3>
{#if tokens.length === 0}
	<EmptyState title={m.gateways_tokens_empty()} />
{:else}
	<Table>
		{#snippet header()}
			<th>{m.gateways_token_col_uses()}</th>
			<th>{m.gateways_token_col_expires()}</th>
			<th></th>
			<th></th>
		{/snippet}
		{#each tokens as t (t.id)}
			<tr>
				<td>
					<CellLead icon="key">
						{m.gateways_token_uses({ uses: t.uses, max_uses: t.maxUses })}
						{#snippet sub()}<Ago ts={t.createdAt} />{/snippet}
					</CellLead>
				</td>
				<td class="muted">{fmt(t.expiresAt)}</td>
				<td>
					{#if t.revokedAt}
						<Badge tone="alert">{m.gateways_token_revoked()}</Badge>
					{:else if expired(t)}
						<Badge tone="ghost">{m.gateways_token_expired()}</Badge>
					{/if}
				</td>
				<td class="more">
					{#if !t.revokedAt}
						<Button scheme="w" variant="plain" onclick={() => onrevoke(t)}>{m.gateways_token_revoke()}</Button>
					{/if}
				</td>
			</tr>
		{/each}
	</Table>
{/if}

<style>
	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.more {
		text-align: right;
	}
</style>
