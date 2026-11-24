<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { Button, Divider, FilledField } from '#lib/ui/index.js';
	import { auth } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { IdentityProviderRef, LoginResponse } from '#lib/gen/werbot/v1/auth_pb.js';
	import LoginMfaStep from '#lib/mfa/LoginMfaStep.svelte';
	import { safeNext } from '#lib/next.js';
	import { m } from '#lib/paraglide/messages.js';

	let email = $state('');
	let password = $state('');
	// set once the password was right and a second factor is due
	let challenge = $state<LoginResponse | null>(null);
	let providers = $state<IdentityProviderRef[]>([]);

	const next = $derived(safeNext(page.url.searchParams.get('next')));

	// a full page load: /api/oidc/ is a server route, not a page of the app
	const ssoStart = (id: string) => `/api/oidc/${id}/start?next=${encodeURIComponent(next)}`;

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			const res = await auth.login({ email, password });
			if (res.mfaRequired) challenge = res;
			else await goto(next);
		} catch (err) {
			failToast(err);
		}
	}

	onMount(async () => {
		if (page.url.searchParams.get('error') === 'oidc') failToast(m.auth_oidc_failed());
		try {
			providers = (await auth.listIdentityProviders({})).providers;
		} catch (err) {
			failToast(err);
		}
	});
</script>

{#if challenge}
	<LoginMfaStep
		mfaToken={challenge.mfaToken}
		methods={challenge.methods}
		webauthnOptions={challenge.webauthnOptions}
		ondone={() => goto(next)}
		onback={() => ((challenge = null), (password = ''))}
	/>
{:else}
	<form onsubmit={submit}>
		<h1>{m.auth_login_title()}</h1>
		<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
		<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="current-password" />
		<Button type="submit" variant="solid" size={48}>{m.auth_login_submit()}</Button>
		<a class="forgot" href="/forgot-password">{m.auth_forgot_link()}</a>
		{#if providers.length > 0}
			<Divider />
			{#each providers as p (p.id)}
				<Button type="button" scheme="g" variant="plain" size={48} onclick={() => location.assign(ssoStart(p.id))}>{m.auth_sso_with({ name: p.name })}</Button>
			{/each}
		{/if}
	</form>
{/if}
