<script lang="ts">
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import Ago from '#lib/Ago.svelte';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import type { Slot } from './dashboard';

	let { events }: { events: Slot<AuditEvent[]> } = $props();
</script>

<DashCard
	title={m.dashboard_recent_audit()}
	icon="report_detailed"
	href="/audit"
	slot={events}
	empty={{ icon: 'report_detailed', title: m.audit_empty() }}
>
	{#snippet children(list)}
		<ul>
			{#each list as ev (ev.id)}
				<!-- events without a user actor are the system's -->
				<DashRow href="/audit" title={ev.action} icon="gear" avatar={ev.actorLogin || undefined}>
					{#snippet meta()}{[ev.actorLogin || ev.actorKind, ev.targetType].filter(Boolean).join(' · ')}{/snippet}
					{#snippet trail()}<Ago ts={ev.createdAt} />{/snippet}
				</DashRow>
			{/each}
		</ul>
	{/snippet}
</DashCard>
