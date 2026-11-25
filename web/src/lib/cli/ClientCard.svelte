<script lang="ts">
	import { Button, Chip, PropertyRow, SettingsCard } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	/** a pending CLI sign-in: the loopback request or the device code */
	interface Client {
		clientId: string;
		clientHostname: string;
		clientIp: string;
		scopes: string[];
	}

	interface Props {
		client: Client;
		hint: (args: { client: string; hostname: string; ip: string }) => string;
		approve: string;
		deny: string;
		/** a decision is in flight: both buttons wait */
		busy?: boolean;
		onapprove: () => void;
		ondeny: () => void;
	}

	let { client, hint, approve, deny, busy = false, onapprove, ondeny }: Props = $props();
</script>

<SettingsCard icon="laptop" title={client.clientHostname || client.clientId}>
	<div class="body">
		<div class="props">
			<PropertyRow label={m.cli_authorize_client()}>{client.clientId}</PropertyRow>
			<PropertyRow label={m.cli_authorize_host()}>{client.clientHostname || '—'}</PropertyRow>
			<PropertyRow label={m.cli_authorize_ip()}>{client.clientIp || '—'}</PropertyRow>
			<PropertyRow label={m.cli_authorize_scopes()}>
				{#each client.scopes as s (s)}<Chip>{s}</Chip>{:else}—{/each}
			</PropertyRow>
		</div>
		<p>{hint({ client: client.clientId, hostname: client.clientHostname || '—', ip: client.clientIp || '—' })}</p>
		<div class="actions">
			<Button size={40} variant="solid" disabled={busy} onclick={onapprove}>{approve}</Button>
			<Button size={40} scheme="g" variant="plain" disabled={busy} onclick={ondeny}>{deny}</Button>
		</div>
	</div>
</SettingsCard>

<style>
	.body {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-16);
	}

	.props {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
	}

	p {
		margin: 0;
	}

	.actions {
		display: flex;
		gap: var(--miso-sp-08);
	}
</style>
