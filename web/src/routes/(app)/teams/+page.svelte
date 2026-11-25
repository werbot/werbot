<script lang="ts">
	import { onMount } from 'svelte';
	import { AvatarStack, Button, EmptyState, Modal, Skeleton, Table, FilledField, toast } from '#lib/ui/index.js';
	import { members, teams } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import RowMenu from '#lib/RowMenu.svelte';
	import CellLead from '#lib/CellLead.svelte';
	import TeamDrawer from '#lib/teams/TeamDrawer.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import PageBody from '#lib/shell/PageBody.svelte';
	import PageHeader from '#lib/shell/PageHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Team[]>([]);
	let people = $state<Member[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let creating = $state(false);
	let newName = $state('');
	let selected = $state('');
	let menuOpen = $state('');
	let removing = $state<Team | null>(null);

	const nameOf = (id: string) => people.find((p) => p.user?.id === id)?.user?.name ?? id;
	// avatars are keyed by login
	const loginOf = (id: string) => people.find((p) => p.user?.id === id)?.user?.login ?? id;
	const current = $derived(list.find((t) => t.id === selected) ?? null);

	async function refresh() {
		[{ teams: list }, { members: people }] = await Promise.all([teams.listTeams({}), members.listMembers({})]);
		loaded = true;
	}

	// a failed load toasts and swaps the list for LoadFailed until a reload succeeds
	const load = () => refresh().then(() => (failed = false), (e) => ((failed = true), failToast(e)));

	async function run(action: () => Promise<unknown>, message = m.common_saved()) {
		try {
			await action();
			toast({ tone: 'success', text: message });
		} catch (e) {
			failToast(e);
			await refresh().catch(() => {});
			return false;
		}
		await load();
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

	onMount(load);
</script>

<PageHeader title={m.teams_title()} count={loaded ? list.length : undefined}>
	{#snippet toolbar()}
		{#if list.length > 0}
			<Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.teams_new()}</Button>
		{/if}
	{/snippet}
</PageHeader>
<PageBody flush>
	{#if failed}
		<LoadFailed onretry={load} />
	{:else if !loaded}
		<Skeleton variant="text" lines={6} />
	{:else if list.length === 0}
		<EmptyState icon="workspace" title={m.teams_empty()}>
			{#snippet actions()}
				<Button variant="solid" size={40} iconLeft="plus" onclick={() => (creating = true)}>{m.teams_new()}</Button>
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
					<td>
						<CellLead icon="people">
							<button class="row-name" onclick={() => (selected = team.id)}>{team.name}</button>
							{#snippet sub()}{m.teams_member_count({ count: team.memberUserIds.length })}{/snippet}
						</CellLead>
					</td>
					<td><AvatarStack people={team.memberUserIds.map((id) => ({ name: loginOf(id) }))} /></td>
					<td class="more">
						<RowMenu label={m.teams_delete()} onselect={() => (removing = team)} onedit={() => (selected = team.id)} bind:open={() => menuOpen === team.id, (v) => (menuOpen = v ? team.id : '')} />
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
	{loginOf}
	onrename={(name) => current && run(() => teams.renameTeam({ id: current.id, name }))}
	onadd={(userId) => current && run(() => teams.addTeamMember({ teamId: current.id, userId }))}
	onremove={(userId) => current && run(() => teams.removeTeamMember({ teamId: current.id, userId }))}
	ondelete={() => (removing = current)}
	onclose={() => {
		if (menuOpen || removing) return;
		selected = '';
	}}
/>

<Modal bind:open={creating} title={m.teams_new()} width={480}>
	<form id="create-team" onsubmit={create}>
		<FilledField label={m.teams_name()} bind:value={newName} required maxlength={64} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (creating = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="create-team" variant="solid">{m.teams_create_submit()}</Button>
	{/snippet}
</Modal>

<Modal open={removing !== null} onclose={() => (removing = null)} title={m.teams_delete()}>
	<p>{m.teams_delete_confirm()}</p>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (removing = null)}>{m.common_cancel()}</Button>
		<Button size={40} scheme="w" variant="solid" onclick={remove}>{m.teams_delete()}</Button>
	{/snippet}
</Modal>

<style>
	.more {
		text-align: right;
	}
</style>
