<script lang="ts">
	import { onMount } from 'svelte';
	import { Button, FilledField, Modal } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { finishStepUp } from '#lib/stepup.svelte.js';
	import { failPasskey, getAssertion } from '#lib/webauthn.js';
	import { m } from '#lib/paraglide/messages.js';

	let methods = $state<string[]>([]);
	let options = $state<Uint8Array>(new Uint8Array());
	let code = $state('');

	// mounted once per step-up: ask the server which factors it can offer
	onMount(() => {
		void auth.beginStepUp({}).then(
			(r) => ((methods = r.methods), (options = r.webauthnOptions)),
			(e) => (failToast(e), finishStepUp(false))
		);
	});

	async function passkey() {
		try {
			await auth.completeStepUp({ proof: { case: 'webauthnAssertion', value: await getAssertion(options) } });
			finishStepUp(true);
		} catch (e) {
			failPasskey(e);
		}
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await auth.completeStepUp({ proof: { case: 'totpCode', value: code.trim() } });
			finishStepUp(true);
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Modal open onclose={() => finishStepUp(false)} title={m.mfa_stepup_title()} width={420}>
	<p>{m.mfa_stepup_hint()}</p>
	{#if methods.includes('webauthn')}
		<Button type="button" variant="solid" size={40} iconLeft="key" onclick={passkey}>{m.auth_mfa_passkey()}</Button>
	{/if}
	{#if methods.includes('totp')}
		<form id="step-up" onsubmit={submit}>
			<FilledField label={m.auth_mfa_totp_code()} bind:value={code} required inputmode="numeric" pattern={'[0-9]{6}'} maxlength={6} autocomplete="one-time-code" />
		</form>
	{/if}
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => finishStepUp(false)}>{m.common_cancel()}</Button>
		{#if methods.includes('totp')}<Button size={40} type="submit" form="step-up" variant="solid">{m.auth_mfa_verify()}</Button>{/if}
	{/snippet}
</Modal>

<style>
	p {
		margin: 0 0 var(--miso-sp-12);
	}

	form {
		margin-top: var(--miso-sp-12);
	}
</style>
