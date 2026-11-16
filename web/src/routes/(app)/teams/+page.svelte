<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner, Button, Chip, EmptyState, InlineEdit, Select, SettingsCard, TextField } from '#lib/ui/index.js';
	import { members, teams } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let list = $state<Team[]>([]);
	let people = $state<Member[]>([]);
	let error = $state('');
	let newName = $state('');

	let byId = $derived(new Map(people.map((p) => [p.user?.id, p.user])));

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			[{ teams: list }, { members: people }] = await Promise.all([teams.listTeams({}), members.listMembers({})]);
			return true;
		} catch (e) {
			error = errorMessage(e);
			return false;
		}
	}

	async function create(e: SubmitEvent) {
		e.preventDefault();
		if (await run(() => teams.createTeam({ name: newName }))) newName = '';
	}

	onMount(() => run(async () => {}));
</script>

<h1>{m.teams_title()}</h1>
{#if error}<Banner tone="alert">{error}</Banner>{/if}

<form class="create" onsubmit={create}>
	<TextField label={m.teams_name()} bind:value={newName} required maxlength={64} />
	<Button type="submit" variant="solid">{m.teams_create_submit()}</Button>
</form>

{#if list.length === 0}
	<EmptyState title={m.teams_empty()} />
{/if}

<div class="cards">
	{#each list as team (team.id)}
		<SettingsCard>
			{#snippet header()}
				<InlineEdit value={team.name} onsave={(name) => run(() => teams.renameTeam({ id: team.id, name }))} />
			{/snippet}
			{#snippet actions()}
				<Button scheme="w" variant="plain" onclick={() => run(() => teams.deleteTeam({ id: team.id }))}>{m.teams_delete()}</Button>
			{/snippet}
			<div class="chips">
				{#each team.memberUserIds as id (id)}
					<Chip deletable ondelete={() => run(() => teams.removeTeamMember({ teamId: team.id, userId: id }))}>
						{byId.get(id)?.name ?? id}
					</Chip>
				{/each}
			</div>
			<Select
				placeholder={m.teams_add_member()}
				options={people
					.filter((p) => !team.memberUserIds.includes(p.user?.id ?? ''))
					.map((p) => ({ value: p.user?.id ?? '', label: p.user?.name ?? '' }))}
				onchange={(userId) => run(() => teams.addTeamMember({ teamId: team.id, userId }))}
			/>
		</SettingsCard>
	{/each}
</div>

<style>
	.create {
		display: flex;
		align-items: flex-end;
		gap: 12px;
		max-width: 480px;
	}

	.cards {
		display: flex;
		flex-direction: column;
		gap: 16px;
		max-width: 640px;
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
		margin-bottom: 12px;
	}
</style>
