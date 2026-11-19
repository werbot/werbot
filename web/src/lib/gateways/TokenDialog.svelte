<script lang="ts">
	import { durationFromMs } from '@bufbuild/protobuf/wkt';
	import { Button, Modal, FilledField } from '#lib/ui/index.js';
	import { gateways } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import { fmt } from '#lib/format.js';

	interface Props {
		gatewayId: string;
		open: boolean;
		oncreated: () => void;
	}

	let { gatewayId, open = $bindable(), oncreated }: Props = $props();

	let hours = $state('24');
	let maxUses = $state('10');
	let copied = $state(false);
	let created = $state<{ token: string; expires: string; maxUses: number }>();

	// env form: a token in argv shows up in ps output and shell history
	let command = $derived(created ? `WERBOT_GATEWAY_JOIN_TOKEN=${created.token} werbot gateway --server ${location.origin}` : '');

	async function create(e: SubmitEvent) {
		e.preventDefault();
		try {
			const res = await gateways.createJoinToken({
				gatewayId,
				expiresIn: durationFromMs(Number(hours) * 3_600_000),
				maxUses: Number(maxUses)
			});
			created = { token: res.token, expires: fmt(res.expiresAt), maxUses: Number(maxUses) };
			oncreated();
		} catch (err) {
			failToast(err);
		}
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(command);
			copied = true;
		} catch {
			failToast(m.gateways_copy_failed());
		}
	}
</script>

<Modal bind:open title={m.gateways_token_create()}>
	{#if created}
		<p>{m.gateways_token_hint({ max_uses: created.maxUses, time: created.expires })}</p>
		<pre><code>{command}</code></pre>
		<Button size={40} onclick={copy}>{copied ? m.gateways_copied() : m.gateways_copy()}</Button>
	{:else}
		<form id="token-form" onsubmit={create}>
			<FilledField label={m.gateways_token_expires_in()} type="number" min={1} max={8760} bind:value={hours} required />
			<FilledField label={m.gateways_token_max_uses()} type="number" min={1} max={1000} bind:value={maxUses} required />
		</form>
	{/if}
	{#snippet footer()}
		{#if !created}<Button size={40} type="submit" form="token-form" variant="solid">{m.gateways_token_create()}</Button>{/if}
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	pre {
		overflow-x: auto;
		user-select: all;
	}
</style>
