<script lang="ts">
	import { Button, Chip, Drawer, InlineEdit, PropertyRow } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		team: Team | null;
		people: Member[];
		nameOf: (id: string) => string;
		onrename: (name: string) => void;
		onadd: (userId: string) => void;
		onremove: (userId: string) => void;
		ondelete: () => void;
		onclose: () => void;
	}

	let { team, people, nameOf, onrename, onadd, onremove, ondelete, onclose }: Props = $props();

	let pick = $state('');

	const candidates = $derived(
		people
			.filter((p) => team && !team.memberUserIds.includes(p.user?.id ?? ''))
			.map((p) => ({ value: p.user?.id ?? '', label: p.user?.name ?? '' }))
	);
</script>

<Drawer bind:open={() => !!team, (v) => !v && onclose()} resizable snapPoints={[480, 720]} width={480} title={m.teams_title()}>
	{#if team}
		<!-- Escape cancels the edit only: stop it before it reaches the drawer's window listener -->
		<div class="head" role="presentation" onkeydown={(e) => e.key === 'Escape' && e.stopPropagation()}>
			<InlineEdit variant="title" value={team.name} onsave={onrename} />
		</div>
		<PropertyRow label={m.members_title()}>
			<div class="chips">
				{#each team.memberUserIds as id (id)}
					<Chip deletable avatar={nameOf(id)} ondelete={() => onremove(id)}>{nameOf(id)}</Chip>
				{/each}
			</div>
		</PropertyRow>
		<FilledSelect label={m.teams_add_member()} options={candidates} bind:value={pick} onchange={(v) => { onadd(v); pick = ''; }} />
	{/if}
	{#snippet footer()}
		<Button scheme="w" variant="solid" size={28} iconLeft="trash" onclick={ondelete}>{m.teams_delete()}</Button>
	{/snippet}
</Drawer>

<style>
	.head {
		margin-bottom: var(--miso-sp-16);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08);
	}
</style>
