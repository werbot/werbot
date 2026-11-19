<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import type { PageProps } from './$types';
	import { Code, ConnectError } from '@connectrpc/connect';
	import { Avatar, Banner, Button, FilledField, InfoBlock, PropertyRow, SettingsCard, toast } from '#lib/ui/index.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { profile } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	let { data }: PageProps = $props();

	const roleLabels = { [Role.OWNER]: m.role_owner, [Role.ADMIN]: m.role_admin, [Role.MEMBER]: m.role_member };
	let name = $state('');
	let login = $state('');
	let createdAt = $state<Timestamp>();
	let email = $state('');
	let savedEmail = $state('');
	let emailPassword = $state('');
	const emailChanged = $derived(email.trim().toLowerCase() !== savedEmail);
	let current = $state('');
	let next = $state('');
	let error = $state('');

	async function run(action: () => Promise<unknown>, message: string) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: message });
			return true;
		} catch (e) {
			error = errorMessage(e);
			return false;
		}
	}

	// A stale tab may hold an old email: re-read it so the password field shows.
	async function refreshSavedEmail() {
		try {
			savedEmail = (await profile.getProfile({})).user?.email ?? savedEmail;
		} catch {
			// keep savedEmail: the save error is already shown
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		error = '';
		try {
			const { user } = await profile.updateProfile({ name, email, currentPassword: emailChanged ? emailPassword : '' });
			savedEmail = user?.email ?? '';
			emailPassword = '';
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			error = errorMessage(e);
			if (ConnectError.from(e).code === Code.InvalidArgument) await refreshSavedEmail();
		}
	}

	async function changePassword(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => profile.changePassword({ currentPassword: current, newPassword: next }), m.common_saved())) {
			current = next = '';
		}
	}

	onMount(async () => {
		try {
			const { user } = await profile.getProfile({});
			name = user?.name ?? '';
			email = savedEmail = user?.email ?? '';
			login = user?.login ?? '';
			createdAt = user?.createdAt;
		} catch (e) {
			error = errorMessage(e);
		}
	});

	async function requestDeletion() {
		error = '';
		try {
			await profile.requestAccountDeletion({});
			toast({ tone: 'success', text: m.profile_delete_sent() });
		} catch (e) {
			error = ConnectError.from(e).code === Code.FailedPrecondition ? m.profile_delete_last_owner() : errorMessage(e);
		}
	}
</script>

<PageHeader title={m.profile_title()} />
<PageBody width={680}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}

	<!-- the context column carries this from 1024px up -->
	<div class="me">
		<Avatar name={name || savedEmail} size={56} />
		<div class="me__text">
			<span class="me__name">{name}</span>
			<span class="me__email">{savedEmail}</span>
		</div>
	</div>

	<SettingsCard icon="person" title={m.profile_personal()}>
		<div class="props">
			<PropertyRow label={m.field_login()}>{login}</PropertyRow>
			<PropertyRow label={m.profile_role()}>{roleLabels[data.session.role as keyof typeof roleLabels]?.() ?? ''}</PropertyRow>
			<PropertyRow label={m.profile_member_since()}>{createdAt ? timestampDate(createdAt).toLocaleDateString() : ''}</PropertyRow>
		</div>
		<form onsubmit={save}>
			<FieldRow even>
				<FilledField label={m.field_name()} bind:value={name} required autocomplete="name" />
				<FilledField type="email" label={m.field_email()} bind:value={email} required autocomplete="email" />
			</FieldRow>
			{#if emailChanged}
				<FieldRow even>
					<FilledField type="password" label={m.profile_email_password()} bind:value={emailPassword} required autocomplete="current-password" />
				</FieldRow>
			{/if}
			<div><Button type="submit" variant="solid">{m.profile_save()}</Button></div>
		</form>
	</SettingsCard>

	<SettingsCard icon="lock" title={m.profile_password_title()}>
		<form onsubmit={changePassword}>
			<FieldRow even>
				<FilledField type="password" label={m.profile_password_current()} bind:value={current} required autocomplete="current-password" />
				<FilledField type="password" label={m.profile_password_new()} bind:value={next} required minlength={12} autocomplete="new-password" />
			</FieldRow>
			<div><Button type="submit" variant="solid">{m.profile_password_submit()}</Button></div>
		</form>
	</SettingsCard>

	<SettingsCard icon="trash_can" title={m.profile_delete_title()}>
		<div class="danger">
			<InfoBlock tone="warning" icon="exclam_triangle-line" title={m.common_warning()}>{m.profile_delete_warning()}</InfoBlock>
			<div><Button scheme="w" variant="solid" onclick={requestDeletion}>{m.profile_delete_submit()}</Button></div>
		</div>
	</SettingsCard>
</PageBody>

<style>
	form,
	.danger {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.props {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
		margin-bottom: var(--miso-sp-16);
	}

	.me {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-16);
	}

	.me__text {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.me__name {
		font: var(--miso-font-ui-16-semi);
		color: var(--miso-txt-accent);
	}

	.me__email {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}

	@media (min-width: 1024px) {
		.me {
			display: none;
		}
	}
</style>
