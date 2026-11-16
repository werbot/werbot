<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Banner, Button, Modal, Select, Table, TextField, toast, type SelectOption } from '#lib/ui/index.js';
	import { members } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import type { Invite, Member } from '#lib/gen/werbot/v1/member_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const roleLabels = { [Role.OWNER]: m.role_owner, [Role.ADMIN]: m.role_admin, [Role.MEMBER]: m.role_member };
	// Only an owner may grant the owner role.
	let roleOptions = $derived(
		([Role.OWNER, Role.ADMIN, Role.MEMBER] as const)
			.filter((role) => role !== Role.OWNER || data.session.role === Role.OWNER)
			.map((role): SelectOption => ({ value: String(role), label: roleLabels[role]() }))
	);

	let list = $state<Member[]>([]);
	let invites = $state<Invite[]>([]);
	let error = $state('');
	let inviting = $state(false);
	let inviteEmail = $state('');
	let inviteRole = $state(String(Role.MEMBER));
	let removing = $state<Member | null>(null);

	const date = (ts?: Parameters<typeof timestampDate>[0]) => (ts ? timestampDate(ts).toLocaleDateString() : '');

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			({ members: list, invites } = await members.listMembers({}));
			return true;
		} catch (e) {
			error = errorMessage(e);
			return false;
		}
	}

	async function invite(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => members.inviteMember({ email: inviteEmail, role: Number(inviteRole) }))) {
			inviting = false;
			inviteEmail = '';
			toast({ tone: 'success', text: m.common_saved() });
		}
	}

	async function remove() {
		const member = removing;
		removing = null;
		if (member?.user) await run(() => members.removeMember({ userId: member.user!.id }));
	}

	onMount(() => run(async () => {}));
</script>

<h1>{m.members_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}
<Button variant="solid" onclick={() => (inviting = true)}>{m.members_invite()}</Button>

<Table>
	{#snippet header()}
		<th>{m.field_name()}</th>
		<th>{m.field_email()}</th>
		<th>{m.field_login()}</th>
		<th>{m.members_role()}</th>
		<th>{m.members_joined()}</th>
		<th></th>
	{/snippet}
	{#each list as member (member.user?.id)}
		<tr>
			<td>{member.user?.name}</td>
			<td>{member.user?.email}</td>
			<td>{member.user?.login}</td>
			<td>
				<Select
					options={roleOptions}
					value={String(member.role)}
					onchange={(value) => run(() => members.updateMemberRole({ userId: member.user!.id, role: Number(value) }))}
				/>
			</td>
			<td>{date(member.joinedAt)}</td>
			<td><Button scheme="w" variant="plain" onclick={() => (removing = member)}>{m.members_remove()}</Button></td>
		</tr>
	{/each}
</Table>

{#if invites.length > 0}
	<h2>{m.members_invites_title()}</h2>
	<Table>
		{#snippet header()}
			<th>{m.field_email()}</th>
			<th>{m.members_role()}</th>
			<th>{m.members_invite_expires()}</th>
		{/snippet}
		{#each invites as item (item.email)}
			<tr>
				<td>{item.email}</td>
				<td>{roleLabels[item.role as keyof typeof roleLabels]?.()}</td>
				<td>{date(item.expiresAt)}</td>
			</tr>
		{/each}
	</Table>
{/if}

<Modal bind:open={inviting} title={m.members_invite()}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="invite" onsubmit={invite}>
		<TextField type="email" label={m.field_email()} bind:value={inviteEmail} required />
		<Select label={m.members_role()} options={roleOptions} bind:value={inviteRole} />
	</form>
	{#snippet footer()}
		<Button variant="plain" onclick={() => (inviting = false)}>{m.common_cancel()}</Button>
		<Button type="submit" form="invite" variant="solid">{m.members_invite_submit()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.members_remove()}>
	<p>{m.members_remove_confirm()}</p>
	{#snippet footer()}
		<Button variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={remove}>{m.members_remove()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
</style>
