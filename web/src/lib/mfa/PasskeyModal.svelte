<script lang="ts">
	import { Button, FilledField, Modal } from '#lib/ui/index.js';
	import { mfa } from '#lib/api.js';
	import { createCredential, failPasskey } from '#lib/webauthn.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		onadded: () => void;
	}

	let { open = $bindable(), onadded }: Props = $props();

	let name = $state(m.mfa_passkey_default_name());
	let busy = $state(false);

	async function add(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			const { options } = await mfa.beginWebAuthnRegistration({});
			const attestation = await createCredential(options);
			await mfa.finishWebAuthnRegistration({ name: name.trim(), attestation });
			open = false;
			onadded();
		} catch (err) {
			failPasskey(err);
		} finally {
			busy = false;
		}
	}
</script>

<Modal bind:open title={m.mfa_passkey_add()} width={440}>
	<form id="add-passkey" onsubmit={add}>
		<p>{m.mfa_passkey_hint()}</p>
		<FilledField label={m.mfa_passkey_name()} bind:value={name} required maxlength={64} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="add-passkey" variant="solid" disabled={busy}>{m.mfa_passkey_add()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	p {
		margin: 0;
	}
</style>
