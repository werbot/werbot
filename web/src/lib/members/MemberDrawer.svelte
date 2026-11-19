<script lang="ts">
	import { Avatar, Button, Drawer, Label, PropertyRow, Select, type SelectOption } from '#lib/ui/index.js';
	import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
	import type { Team } from '#lib/gen/werbot/v1/team_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		member: Member | null;
		teams: Team[];
		locked: boolean;
		roleOptions: SelectOption[];
		joined: string;
		roleLabel: (role: number) => string;
		onremove: () => void;
		onrole: (role: number) => void;
		onclose: () => void;
	}

	let { member, teams, locked, roleOptions, joined, roleLabel, onremove, onrole, onclose }: Props = $props();

	// freeze member and its derived props while the close animation plays
	const shown = keepLast(() => member && { member, locked, joined });
	const user = $derived(shown.value?.member.user);
	const teamNames = $derived(teams.filter((t) => user && t.memberUserIds.includes(user.id)).map((t) => t.name));
</script>

<Drawer bind:open={() => !!member, (v) => !v && !selectOpen() && onclose()} resizable snapPoints={[480, 720]} width={480} title={user?.name}>
	{#if shown.value && user}
		{@const { member, locked, joined } = shown.value}
		<div class="head"><Avatar name={user.name} size={40} /></div>
		<PropertyRow label={m.field_email()}>{user.email}</PropertyRow>
		<PropertyRow label={m.field_login()}>{user.login}</PropertyRow>
		<PropertyRow label={m.members_role()}>
			{#if locked}
				<Label variant="soft">{roleLabel(member.role)}</Label>
			{:else}
				<!-- remount on refetch so a rejected pick does not linger -->
				{#key member}
					<Select size={28} options={roleOptions} value={String(member.role)} onchange={(v) => onrole(Number(v))} />
				{/key}
			{/if}
		</PropertyRow>
		<PropertyRow label={m.members_joined()}>{joined || '—'}</PropertyRow>
		<PropertyRow label={m.teams_title()}>
			{#each teamNames as name (name)}<Label variant="soft">{name}</Label>{:else}—{/each}
		</PropertyRow>
	{/if}
	{#snippet footer()}
		<Button scheme="w" variant="solid" size={40} iconLeft="trash" disabled={shown.value?.locked ?? locked} onclick={onremove}>{m.members_remove()}</Button>
	{/snippet}
</Drawer>

<style>
	.head {
		margin-bottom: var(--miso-sp-16);
	}
</style>
