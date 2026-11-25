<script lang="ts">
	import { Button, Checkbox } from '#lib/ui/index.js';
	import { failToast } from '#lib/errors.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import FormField from '#lib/FormField.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { IdpDraft } from './idpDraft.js';

	interface Props {
		/** the form element id: the drawer footer submits it */
		id: string;
		draft: IdpDraft;
		/** the redirect URI to register at the provider; empty for a provider not saved yet */
		redirectUri: string;
		/** an existing provider: the stored client secret is kept when the field is empty */
		existing: boolean;
		onsubmit: () => void;
	}

	let { id, draft = $bindable(), redirectUri, existing, onsubmit }: Props = $props();

	let copied = $state(false);

	async function copy() {
		try {
			await navigator.clipboard.writeText(redirectUri);
			copied = true;
		} catch {
			failToast(m.gateways_copy_failed());
		}
	}
</script>

<form
	{id}
	onsubmit={(e) => {
		e.preventDefault();
		onsubmit();
	}}
>
	<FormField label={m.sso_name()} bind:value={draft.name} required maxlength={63} />
	<FormField label={m.sso_issuer()} hint={m.sso_issuer_hint()} bind:value={draft.issuer} type="url" required placeholder="https://idp.example.com/realms/acme" />
	<FieldRow even>
		<FormField label={m.sso_client_id()} bind:value={draft.clientId} required maxlength={255} />
		<FormField
			label={m.sso_client_secret()}
			hint={existing ? m.sso_client_secret_keep() : undefined}
			bind:value={draft.clientSecret}
			type="password"
			required={!existing}
			autocomplete="off"
			maxlength={1024}
		/>
	</FieldRow>
	<FormField label={m.sso_scopes()} hint={m.sso_scopes_hint()} bind:value={draft.scopes} />

	<div class="checks">
		<Checkbox bind:checked={draft.enabled}>{m.sso_enabled()}</Checkbox>
		<Checkbox bind:checked={draft.allowSignup}>{m.sso_allow_signup()}</Checkbox>
	</div>

	<div class="redirect">
		{#if redirectUri}
			<FormField label={m.sso_redirect_uri()} hint={m.sso_redirect_uri_hint()} value={redirectUri} readonly />
			<Button type="button" size={40} scheme="g" variant="plain" onclick={copy}>{copied ? m.gateways_copied() : m.gateways_copy()}</Button>
		{:else}
			<p class="hint">{m.sso_redirect_uri_after_save()}</p>
		{/if}
	</div>
</form>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.checks {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08) var(--miso-sp-24);
	}

	.redirect {
		display: flex;
		align-items: flex-start;
		gap: var(--miso-sp-12);
	}

	.redirect :global(.field) {
		flex: 1;
	}

	.hint {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
