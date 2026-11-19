<script lang="ts">
	import { Avatar, Badge, Label } from '#lib/ui/index.js';
	import { elapsed } from '#lib/format.js';
	import type { Session } from '#lib/gen/werbot/v1/session_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import Ago from './Ago.svelte';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import type { Slot } from './dashboard';

	let { recent }: { recent: Slot<Session[]> } = $props();
</script>

<DashCard
	title={m.dashboard_recent_sessions()}
	icon="history"
	href="/sessions"
	slot={recent}
	empty={{ icon: 'history', title: m.dashboard_no_sessions() }}
>
	{#snippet children(list)}
		<ul>
			{#each list as s (s.id)}
				<DashRow href={`/sessions/${s.id}`} title={s.resourceName || s.id}>
					{#snippet lead()}<Avatar name={s.userLogin} size={36} online={!s.endedAt} />{/snippet}
					{#snippet meta()}
						<span>{s.userLogin || '—'}</span>
						{#if s.kind}<Label variant="soft" size={16}>{s.kind}</Label>{/if}
					{/snippet}
					{#snippet trail()}
						{#if s.endedAt}
							<span class="span">{elapsed(s.startedAt, s.endedAt)}</span>
						{:else}
							<Badge tone="accent">{m.sessions_status_active()}</Badge>
						{/if}
						<Ago ts={s.startedAt} />
					{/snippet}
				</DashRow>
			{/each}
		</ul>
	{/snippet}
</DashCard>

<style>
	.span {
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}
</style>
