<script lang="ts">
	import { Button, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { failPasskey, getAssertion } from '#lib/webauthn.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		mfaToken: string;
		/** "webauthn", "totp", "recovery": the factors the user has, in the order to offer them */
		methods: string[];
		webauthnOptions: Uint8Array;
		ondone: () => void;
		onback: () => void;
	}

	let { mfaToken, methods, webauthnOptions, ondone, onback }: Props = $props();

	// svelte-ignore state_referenced_locally
	let mode = $state(methods[0] ?? 'totp');
	let code = $state('');

	const titles: Record<string, () => string> = {
		webauthn: m.auth_mfa_use_passkey,
		totp: m.auth_mfa_use_totp,
		recovery: m.auth_mfa_use_recovery
	};
	const others = $derived(methods.filter((x) => x !== mode));

	async function complete(proof: Parameters<typeof auth.completeMFA>[0]['proof']) {
		await auth.completeMFA({ mfaToken, proof });
		ondone();
	}

	async function passkey() {
		try {
			await complete({ case: 'webauthnAssertion', value: await getAssertion(webauthnOptions) });
		} catch (e) {
			failPasskey(e);
		}
	}

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await complete(mode === 'recovery' ? { case: 'recoveryCode', value: code.trim() } : { case: 'totpCode', value: code.trim() });
		} catch (err) {
			failToast(err);
		}
	}

	function switchTo(next: string) {
		mode = next;
		code = '';
	}
</script>

<form onsubmit={submit}>
	<h1>{m.auth_mfa_title()}</h1>
	{#if mode === 'webauthn'}
		<p class="hint">{m.auth_mfa_passkey_hint()}</p>
		<Button type="button" variant="solid" size={48} iconLeft="key" onclick={passkey}>{m.auth_mfa_passkey()}</Button>
	{:else}
		<p class="hint">{mode === 'recovery' ? m.auth_mfa_recovery_hint() : m.auth_mfa_totp_hint()}</p>
		{#if mode === 'recovery'}
			<FilledField label={m.auth_mfa_recovery_code()} bind:value={code} required autocomplete="off" autocapitalize="off" spellcheck={false} />
		{:else}
			<FilledField label={m.auth_mfa_totp_code()} bind:value={code} required inputmode="numeric" pattern={'[0-9]{6}'} maxlength={6} autocomplete="one-time-code" />
		{/if}
		<Button type="submit" variant="solid" size={48}>{m.auth_mfa_verify()}</Button>
	{/if}
	{#each others as other (other)}
		<button type="button" class="forgot" onclick={() => switchTo(other)}>{titles[other]?.()}</button>
	{/each}
	<button type="button" class="forgot" onclick={onback}>{m.auth_mfa_back()}</button>
</form>

<style>
	.hint {
		margin: 0;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}

	/* the public layout styles `.forgot` links; the same look for these buttons */
	button.forgot {
		background: none;
		border: 0;
		padding: 0;
		cursor: pointer;
	}
</style>
