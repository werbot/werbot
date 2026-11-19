<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { Banner, Button, FilledField } from '#lib/ui/index.js';
	import { members } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	let login = $state('');
	let name = $state('');
	let password = $state('');

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			await members.acceptInvite({ token: page.params.token, login, name, password });
			await goto('/');
		} catch (err) {
			failToast(err);
		}
	}
</script>

<form onsubmit={submit}>
	<h1>{m.invite_title()}</h1>
	{#if !data.hasSession}
		<Banner tone="info">{m.invite_existing_hint()}</Banner>
		<FilledField label={m.field_login()} bind:value={login} autocomplete="username" />
		<FilledField label={m.field_name()} bind:value={name} autocomplete="name" />
		<FilledField type="password" label={m.field_password()} bind:value={password} required autocomplete="current-password" />
	{/if}
	<Button type="submit" variant="solid" size={48}>{m.invite_accept_submit()}</Button>
</form>
