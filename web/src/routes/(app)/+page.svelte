<script lang="ts">
	import { onMount } from 'svelte';
	import { Avatar, Badge, ProgressBar, Skeleton } from '#lib/ui/index.js';
	import { audit, gateways, resources, sessions, sshKeys } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import { fmt } from '#lib/format.js';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { type Session, SessionStatus } from '#lib/gen/werbot/v1/session_pb.js';
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { serverInfo } from '#lib/version.js';
	import DashCard from './DashCard.svelte';
	import DashCounters from './DashCounters.svelte';
	import { ACTIVE_PAGE, activeLabel, poolsOnline, unhealthy } from './dashboard';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	// a card is pending until its call settles; a failed call only breaks its own card
	type Slot<T> = { state: 'loading' } | { state: 'error'; text: string } | { state: 'ok'; value: T };
	const loading = { state: 'loading' } as const;
	const load = <T,>(p: Promise<T>, set: (s: Slot<T>) => void) =>
		p.then(
			(value) => set({ state: 'ok', value }),
			(e) => set({ state: 'error', text: errorMessage(e) })
		);

	const manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	const date = new Date().toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' });

	const info = serverInfo();
	let active = $state<Slot<Session[]>>(loading);
	let recent = $state<Slot<Session[]>>(loading);
	let res = $state<Slot<Resource[]>>(loading);
	let pools = $state<Slot<Gateway[]>>(loading);
	let events = $state<Slot<AuditEvent[]>>(loading);
	let keys = $state<Slot<SSHKey[]>>(loading);

	onMount(() => {
		const list = (status: SessionStatus, pageSize: number) => sessions.listSessions({ status, pageSize }).then((r) => r.sessions);
		load(list(SessionStatus.ACTIVE, ACTIVE_PAGE), (s) => (active = s));
		load(list(SessionStatus.UNSPECIFIED, 5), (s) => (recent = s));
		if (manager) {
			load(resources.listResources({}).then((r) => r.resources), (s) => (res = s));
			load(gateways.listGateways({}).then((r) => r.gateways), (s) => (pools = s));
			load(audit.listEvents({ pageSize: 5 }).then((r) => r.events), (s) => (events = s));
		} else {
			load(sshKeys.listSSHKeys({}).then((r) => r.keys), (s) => (keys = s));
		}
	});

	const online = $derived(pools.state === 'ok' ? poolsOnline(pools.value) : undefined);

	// a failed call shows '—' instead of a number
	const counters = $derived.by(() => {
		const num = <T,>(s: Slot<T>, f: (v: T) => string) => (s.state === 'ok' ? f(s.value) : '—');
		return manager
			? [
					{ loading: active.state === 'loading', value: num(active, (v) => activeLabel(v.length)), label: m.dashboard_active_sessions() },
					{ loading: res.state === 'loading', value: num(res, (v) => String(v.length)), label: m.dashboard_resources() },
					{ loading: pools.state === 'loading', value: num(pools, (v) => `${poolsOnline(v).online}/${v.length}`), label: m.dashboard_pools_online() }
				]
			: [
					{ loading: active.state === 'loading', value: num(active, (v) => activeLabel(v.length)), label: m.dashboard_active_sessions() },
					{ loading: keys.state === 'loading', value: num(keys, (v) => String(v.length)), label: m.dashboard_ssh_keys() }
				];
	});
</script>

{#snippet pending(s: Slot<unknown>)}
	{#if s.state === 'loading'}<Skeleton variant="text" lines={3} />{:else if s.state === 'error'}<p class="error">{m.dashboard_load_failed()}: {s.text}</p>{/if}
{/snippet}

<div class="topbar"><span class="topbar__org">{m.nav_dashboard()}</span></div>

<div class="scroll">
<header class="head">
	<p class="date">{date}</p>
	<h1 class="title">{data.session.organization?.name ?? m.dashboard_title()}</h1>
	<p class="version">{info.failed ? m.version_unavailable() : info.version && m.version_label({ version: info.version })}</p>
	<DashCounters {counters} />
</header>

<div class="grid">
	<DashCard title={m.dashboard_recent_sessions()} href="/sessions" empty={recent.state === 'ok' && recent.value.length === 0 ? m.dashboard_no_sessions() : undefined}>
		{#if recent.state === 'ok'}
			<ul class="rows">
				{#each recent.value as s (s.id)}
					<li>
						<a class="row" href={`/sessions/${s.id}`}>
							<Avatar name={s.userLogin} size={24} />
							<span class="row__main">{s.resourceName || s.id}</span>
							<Badge tone={s.endedAt ? 'ghost' : 'accent'}>{s.endedAt ? m.sessions_status_ended() : m.sessions_status_active()}</Badge>
						</a>
					</li>
				{/each}
			</ul>
		{:else}
			{@render pending(recent)}
		{/if}
	</DashCard>

	{#if manager}
		<DashCard title={m.dashboard_resource_health()} href="/resources" empty={res.state === 'ok' && unhealthy(res.value).length === 0 ? m.dashboard_all_healthy() : undefined}>
			{#if res.state === 'ok'}
				<ul class="rows">
					{#each unhealthy(res.value) as r (r.id)}
						<li>
							<a class="row" href={`/resources/${r.id}`}>
								<span class="row__main">{r.name}</span>
								<span class="row__err" title={r.health?.error}><Badge tone="alert">{r.health?.error}</Badge></span>
							</a>
						</li>
					{/each}
				</ul>
			{:else}
				{@render pending(res)}
			{/if}
		</DashCard>

		<DashCard title={m.dashboard_pools()} href="/gateways" empty={online?.total === 0 ? m.gateways_empty() : undefined}>
			{#if online}
				<ProgressBar value={online.online / online.total} />
				<ul class="rows">
					{#each pools.state === 'ok' ? pools.value : [] as g (g.id)}
						<li>
							<a class="row" href="/gateways">
								<span class="row__main">{g.name}</span>
								<Badge tone={g.online ? 'accent' : 'ghost'}>{g.online ? m.gateways_online() : m.gateways_offline()}</Badge>
							</a>
						</li>
					{/each}
				</ul>
			{:else}
				{@render pending(pools)}
			{/if}
		</DashCard>

		<DashCard title={m.dashboard_recent_audit()} href="/audit" empty={events.state === 'ok' && events.value.length === 0 ? m.audit_empty() : undefined}>
			{#if events.state === 'ok'}
				<ul class="rows">
					{#each events.value as ev (ev.id)}
						<li>
							<a class="row" href="/audit">
								<Avatar name={ev.actorLogin || ev.actorKind} size={24} />
								<span class="row__main">{ev.action}</span>
								<span class="row__time">{fmt(ev.createdAt)}</span>
							</a>
						</li>
					{/each}
				</ul>
			{:else}
				{@render pending(events)}
			{/if}
		</DashCard>
	{:else}
		<DashCard title={m.dashboard_ssh_keys()} href="/profile/keys" empty={keys.state === 'ok' && keys.value.length === 0 ? m.dashboard_no_keys() : undefined}>
			{#if keys.state === 'ok'}
				<a class="row" href="/profile/keys"><span class="row__main">{m.dashboard_ssh_keys_count({ count: keys.value.length })}</span></a>
			{:else}
				{@render pending(keys)}
			{/if}
		</DashCard>
	{/if}
</div>
</div>

<style>
	.topbar {
		display: flex;
		align-items: center;
		box-sizing: content-box;
		height: 40px;
		padding: 12px 32px;
	}
	.topbar__org {
		font: var(--miso-font-ui-22-semi);
		color: var(--miso-txt-accent);
	}
	.scroll {
		padding: 0 32px 48px;
	}
	.head {
		display: flex;
		flex-direction: column;
		align-items: center;
		max-width: 928px;
		margin: 0 auto;
	}
	.date {
		margin: 6px 0;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}
	.title {
		margin: 0;
		font: var(--miso-font-ui-32-med);
		color: var(--miso-txt-primary);
	}
	.version {
		margin: var(--miso-sp-04) 0 0;
		font: var(--miso-font-ui-13-reg);
		color: var(--miso-txt-secondary);
	}
	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(min(100%, 28rem), 1fr));
		gap: var(--miso-sp-24);
		max-width: 1600px;
		margin: 0 auto;
	}
	.rows {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.row {
		display: flex;
		align-items: center;
		gap: var(--miso-sp-08);
		min-width: 0;
		color: inherit;
		text-decoration: none;
	}
	.row__main {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.row__err {
		min-width: 0;
		max-width: 60%;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.row__time {
		color: var(--miso-txt-secondary);
		font: var(--miso-font-ui-12-reg);
	}
	.error {
		margin: 0;
		color: var(--miso-txt-secondary);
	}
</style>