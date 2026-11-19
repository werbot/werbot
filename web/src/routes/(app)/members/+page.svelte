<script lang="ts">
	import { onMount } from 'svelte';
	import { timestampDate } from '@bufbuild/protobuf/wkt';
	import { Avatar, Banner, Button, Dropdown, EmptyState, Label, Menu, MenuItem, Modal, Select, Skeleton, Table, Tabs, TextField, Tooltip, toast, type SelectOption } from '#lib/ui/index.js';
	import { members, teams as teamsApi } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import type { Invite, Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import MemberDrawer from '#lib/members/MemberDrawer.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
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
	let teams = $state<Team[]>([]);
	let loaded = $state(false);
	let tab = $state('members');
	let selected = $state('');
	let menuOpen = $state('');
	let error = $state('');
	let inviting = $state(false);
	let inviteEmail = $state('');
	let inviteRole = $state(String(Role.MEMBER));
	let removing = $state<Member | null>(null);

	const date = (ts?: Parameters<typeof timestampDate>[0]) => (ts ? timestampDate(ts).toLocaleDateString() : '');

	async function refresh() {
		const [res, teamsRes] = await Promise.all([members.listMembers({}), teamsApi.listTeams({}).catch(() => ({ teams: [] as Team[] }))]);
		({ members: list, invites } = res);
		({ teams } = teamsRes);
		loaded = true;
	}

	async function run(action: () => Promise<unknown>, message = m.common_saved()) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: message });
		} catch (e) {
			error = errorMessage(e);
			// the page banner sits behind the drawer overlay
			if (selected) toast({ tone: 'alert', text: error });
			await refresh().catch(() => {});
			return false;
		}
		await refresh().catch((e) => (error = errorMessage(e)));
		return true;
	}

	// Only an owner may change or remove an owner.
	const locked = (member: Member) => member.role === Role.OWNER && data.session.role !== Role.OWNER;

	async function invite(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => members.inviteMember({ email: inviteEmail, role: Number(inviteRole) }), m.members_invite_sent())) {
			inviting = false;
			inviteEmail = '';
		}
	}

	async function remove() {
		const member = removing;
		removing = null;
		if (member?.user) await run(() => members.removeMember({ userId: member.user!.id }));
	}

	const setRole = (member: Member, role: number) => run(() => members.updateMemberRole({ userId: member.user!.id, role }));
	const current = $derived(list.find((x) => x.user?.id === selected) ?? null);
	const tabOptions = $derived([
		{ value: 'members', label: m.members_title(), count: list.length },
		{ value: 'invites', label: m.members_invites_title(), count: invites.length }
	]);

	// the open member vanished from a refetch (removed): close the drawer
	$effect(() => {
		if (loaded && selected && !current) selected = '';
	});

	onMount(() => refresh().catch((e) => (error = errorMessage(e))));
</script>

<PageHeader title={m.members_title()}>
	{#snippet toolbar()}
		<Button variant="solid" size={28} iconLeft="plus" onclick={() => (inviting = true)}>{m.members_invite()}</Button>
	{/snippet}
	{#snippet tabs()}
		<Tabs options={tabOptions} bind:value={tab} />
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if error && !inviting}<Banner tone="alert">{error}</Banner>{/if}

	{#if !loaded}
		{#if !error}<Skeleton variant="text" lines={6} />{/if}
	{:else if tab === 'members'}
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
					<td>
						<button class="name" onclick={() => (selected = member.user?.id ?? '')}>
							<Avatar name={member.user?.name} size={24} />{member.user?.name}
						</button>
					</td>
					<td>{member.user?.email}</td>
					<td>{member.user?.login}</td>
					<td>
						{#if locked(member)}
							<Label variant="soft">{roleLabels[member.role as keyof typeof roleLabels]()}</Label>
						{:else}
							<!-- remount on refetch so a rejected pick does not linger -->
							{#key member}
								<Select size={28} options={roleOptions} value={String(member.role)} onchange={(value) => setRole(member, Number(value))} />
							{/key}
						{/if}
					</td>
					<td>{date(member.joinedAt)}</td>
					<td class="more">
						<Dropdown placement="bottom-end" bind:open={() => menuOpen === member.user?.id, (v) => (menuOpen = v ? (member.user?.id ?? '') : '')}>
							{#snippet trigger({ toggle })}
								<Tooltip text={m.common_more()}>
									<Button scheme="g" variant="ghost" size={28} icon="more-horiz" aria-label={m.common_more()} onclick={toggle} />
								</Tooltip>
							{/snippet}
							{#snippet children({ close })}
								<Menu>
									<MenuItem
										icon="trash"
										danger
										disabled={locked(member)}
										onclick={() => {
											close();
											removing = member;
										}}
									>
										{m.members_remove()}
									</MenuItem>
								</Menu>
							{/snippet}
						</Dropdown>
					</td>
				</tr>
			{/each}
		</Table>
	{:else if invites.length === 0}
		<EmptyState icon="people" title={m.members_invites_empty()} />
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.field_email()}</th>
				<th>{m.members_role()}</th>
				<th>{m.members_invite_expires()}</th>
			{/snippet}
			{#each invites as item (item.email)}
				<tr>
					<td>{item.email}</td>
					<td><Label variant="soft">{roleLabels[item.role as keyof typeof roleLabels]?.()}</Label></td>
					<td>{date(item.expiresAt)}</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while the remove confirm or a menu is open -->
<MemberDrawer
	member={current}
	{teams}
	locked={current ? locked(current) : false}
	{roleOptions}
	joined={current ? date(current.joinedAt) : ''}
	roleLabel={(role) => roleLabels[role as keyof typeof roleLabels]()}
	onremove={() => (removing = current)}
	onrole={(role) => current && setRole(current, role)}
	onclose={() => !removing && !menuOpen && (selected = '')}
/>

<Modal bind:open={inviting} title={m.members_invite()} width={480}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="invite" onsubmit={invite}>
		<TextField type="email" label={m.field_email()} bind:value={inviteEmail} required />
		<Select label={m.members_role()} options={roleOptions} bind:value={inviteRole} />
	</form>
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => (inviting = false)}>{m.common_cancel()}</Button>
		<Button type="submit" form="invite" variant="solid">{m.members_invite_submit()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.members_remove()}>
	<p>{m.members_remove_confirm()}</p>
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={remove}>{m.members_remove()}</Button>
	{/snippet}
</Modal>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}

	.name {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-08);
		font-weight: 600;
		background: none;
		border: 0;
		padding: 0;
		cursor: pointer;
		color: inherit;
	}

	.name:focus-visible {
		outline: 2px solid var(--miso-line-accent);
		outline-offset: 2px;
		border-radius: var(--miso-radius-04);
	}

	.more {
		text-align: right;
	}
</style>
