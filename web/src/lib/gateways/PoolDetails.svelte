<script lang="ts">
	import { onMount } from 'svelte';
	import { untrack } from 'svelte';
	import { Badge, Button, Chip, PropertyRow, Tooltip, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { gateways } from '#lib/api.js';
	import { failToast, serverText } from '#lib/errors.js';
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
	// the drawer mounts this per pool; the 10 s list poll must not overwrite what the person types
	let address = $state(untrack(() => pool.sshAddress));
	let wsUrl = $state(untrack(() => pool.wsUrl));

	async function saveAddress(e: SubmitEvent) {
		e.preventDefault();
		try {
			// the update replaces both addresses
			const { gateway } = await gateways.updateGateway({ id: pool.id, sshAddress: address.trim(), wsUrl: wsUrl.trim() });
			address = gateway?.sshAddress ?? '';
			wsUrl = gateway?.wsUrl ?? '';
			toast({ tone: 'success', text: m.common_saved() });
		} catch (err) {
			failToast(serverText(err));
		}
	}

	async function copyHostKey() {
		try {
			await navigator.clipboard.writeText(pool.hostKey);
			toast({ tone: 'success', text: m.gateways_copied() });
		} catch {
			failToast(m.gateways_copy_failed());
		}
	}

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
		<PropertyRow label={m.gateways_host_key()}>
			<code class="host-key">{pool.hostKey}</code>
			<Tooltip text={m.gateways_host_key_copy()}>
				<Button scheme="g" variant="ghost" size={28} icon="content_copy" aria-label={m.gateways_host_key_copy()} onclick={copyHostKey} />
			</Tooltip>
		</PropertyRow>
	</div>
	<form class="address" onsubmit={saveAddress}>
		<FormField label={m.gateways_ssh_address_label()} hint={m.gateways_ssh_address_hint()} placeholder="gw.example.com:2222" bind:value={address} maxlength={300} />
		<FormField label={m.gateways_ws_url_label()} hint={m.gateways_ws_url_help()} placeholder="wss://gw.example.com/v1/ssh" bind:value={wsUrl} maxlength={2048} />
		<div><Button size={40} type="submit" variant="solid">{m.gateways_ssh_address_save()}</Button></div>
	</form>
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

	.host-key {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		font-family: var(--miso-font-mono);
	}

	.address {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	section :global(h3) {
		margin: 0 0 var(--miso-sp-08);
	}
</style>
