<script lang="ts">
	import { onMount } from 'svelte';
	import { AvatarStack, Banner, Button, EmptyState, Modal, Skeleton, Table, TextField, toast } from '#lib/ui/index.js';
	import { members, teams } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import RowMenu from '#lib/RowMenu.svelte';
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
	let removing = $state<Team | null>(null);

	const nameOf = (id: string) => people.find((p) => p.user?.id === id)?.user?.name ?? id;
	const current = $derived(list.find((t) => t.id === selected) ?? null);

	async function refresh() {
		[{ teams: list }, { members: people }] = await Promise.all([teams.listTeams({}), members.listMembers({})]);
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

	async function create(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => teams.createTeam({ name: newName }))) {
			creating = false;
			newName = '';
		}
	}

	async function remove() {
		const team = removing;
		removing = null;
		if (team) await run(() => teams.deleteTeam({ id: team.id }), m.common_deleted());
	}

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
					<td><button class="row-name" onclick={() => (selected = team.id)}>{team.name}</button></td>
					<td><AvatarStack people={team.memberUserIds.map((id) => ({ name: nameOf(id) }))} /></td>
					<td class="more">
						<RowMenu label={m.teams_delete()} onselect={() => (removing = team)} bind:open={() => menuOpen === team.id, (v) => (menuOpen = v ? team.id : '')} />
					</td>
				</tr>
			{/each}
		</Table>
	{/if}
</PageBody>

<!-- Escape reaches the drawer too: ignore it while the delete confirm or a menu is open -->
<TeamDrawer
	team={current}
	{people}
	{nameOf}
	onrename={(name) => current && run(() => teams.renameTeam({ id: current.id, name }))}
	onadd={(userId) => current && run(() => teams.addTeamMember({ teamId: current.id, userId }))}
	onremove={(userId) => current && run(() => teams.removeTeamMember({ teamId: current.id, userId }))}
	ondelete={() => (removing = current)}
	onclose={() => {
		if (menuOpen || removing) return;
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

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.teams_delete()}>
	<p>{m.teams_delete_confirm()}</p>
	{#snippet footer()}
		<Button scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button scheme="w" variant="solid" onclick={remove}>{m.teams_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.more {
		text-align: right;
	}
</style>
