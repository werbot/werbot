<script lang="ts">
	import { onMount } from 'svelte';
	import { AvatarStack, Banner, Button, Dropdown, EmptyState, Menu, MenuItem, Modal, Skeleton, Table, TextField, Tooltip, toast } from '#lib/ui/index.js';
	import { members, teams } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import TeamDrawer from '#lib/teams/TeamDrawer.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Team[]>([]);
	let people = $state<Member[]>([]);
	let loaded = $state(false);
	let error = $state('');
	let creating = $state(false);
	let newName = $state('');
	let selected = $state('');
	let menuOpen = $state('');

	const nameOf = (id: string) => people.find((p) => p.user?.id === id)?.user?.name ?? id;
	const current = $derived(list.find((t) => t.id === selected) ?? null);

	async function refresh() {
		[{ teams: list }, { members: people }] = await Promise.all([teams.listTeams({}), members.listMembers({})]);
		loaded = true;
	}

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			toast({ tone: 'success', text: m.common_saved() });
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

	async function create(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => teams.createTeam({ name: newName }))) {
			creating = false;
			newName = '';
		}
	}

	const remove = (team: Team) => run(() => teams.deleteTeam({ id: team.id }));

	// the open team vanished from a refetch (deleted): close the drawer
	$effect(() => {
		if (loaded && selected && !current) selected = '';
	});

	onMount(() => refresh().catch((e) => (error = errorMessage(e))));
</script>

<PageHeader title={m.teams_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.teams_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if error && !creating}<Banner tone="alert">{error}</Banner>{/if}

	{#if !loaded}
		{#if !error}<Skeleton variant="text" lines={6} />{/if}
	{:else if list.length === 0}
		<EmptyState icon="workspace" title={m.teams_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={28} iconLeft="plus" onclick={() => (creating = true)}>{m.teams_new()}</Button>
			{/snippet}
		</EmptyState>
	{:else}
		<Table>
			{#snippet header()}
				<th>{m.field_name()}</th>
				<th>{m.members_title()}</th>
				<th></th>
			{/snippet}
			{#each list as team (team.id)}
				<tr>
					<td><button class="name" onclick={() => (selected = team.id)}>{team.name}</button></td>
					<td><AvatarStack people={team.memberUserIds.map((id) => ({ name: nameOf(id) }))} /></td>
					<td class="more">
						<Dropdown placement="bottom-end" bind:open={() => menuOpen === team.id, (v) => (menuOpen = v ? team.id : '')}>
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
										onclick={() => {
											close();
											remove(team);
										}}
									>
										{m.teams_delete()}
									</MenuItem>
								</Menu>
							{/snippet}
						</Dropdown>
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while a menu is open -->
<TeamDrawer
	team={current}
	{people}
	{nameOf}
	onrename={(name) => current && run(() => teams.renameTeam({ id: current.id, name }))}
	onadd={(userId) => current && run(() => teams.addTeamMember({ teamId: current.id, userId }))}
	onremove={(userId) => current && run(() => teams.removeTeamMember({ teamId: current.id, userId }))}
	ondelete={() => current && remove(current)}
	onclose={() => {
		if (menuOpen) return;
		selected = '';
		error = '';
	}}
/>

<Modal bind:open={() => creating, (v) => ((creating = v), !v && (error = ''))} title={m.teams_new()} width={480}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	<form id="create-team" onsubmit={create}>
		<TextField label={m.teams_name()} bind:value={newName} required maxlength={64} />
	</form>
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => ((creating = false), (error = ''))}>{m.common_cancel()}</Button>
		<Button type="submit" form="create-team" variant="solid">{m.teams_create_submit()}</Button>
	{/snippet}
</Modal>

<style>
	.name {
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
