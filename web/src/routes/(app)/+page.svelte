<script lang="ts">
	import { onMount } from 'svelte';
	import { audit, gateways, resources, sessions, sshKeys } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import type { Gateway } from '#lib/gen/werbot/v1/gateway_pb.js';
	import type { AccessibleResource, Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { type Session, SessionStatus } from '#lib/gen/werbot/v1/session_pb.js';
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { Role } from '#lib/gen/werbot/v1/user_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { serverInfo } from '#lib/version.js';
	import DashCounters from '#lib/dashboard/DashCounters.svelte';
	import EventsCard from '#lib/dashboard/EventsCard.svelte';
	import HealthCard from '#lib/dashboard/HealthCard.svelte';
	import KeysCard from '#lib/dashboard/KeysCard.svelte';
	import PoolsCard from '#lib/dashboard/PoolsCard.svelte';
	import SessionsCard from '#lib/dashboard/SessionsCard.svelte';
	import TerminalsCard from '#lib/dashboard/TerminalsCard.svelte';
	import { ACTIVE_PAGE, activeLabel, onlineCount, type Slot } from '#lib/dashboard/dashboard.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const loading = { state: 'loading' } as const;
	const load = <T,>(get: () => Promise<T>, set: (s: Slot<T>) => void): void => {
		set(loading);
		get().then(
			(value) => set({ state: 'ok', value }),
			(e) => {
				failToast(e);
				set({ state: 'error', retry: () => load(get, set) });
			}
		);
	};

	const manager = $derived(data.session.role === Role.OWNER || data.session.role === Role.ADMIN);
	const date = new Date().toLocaleDateString(undefined, { weekday: 'long', day: 'numeric', month: 'long' });

	const info = serverInfo();
	let active = $state<Slot<Session[]>>(loading);
	let recent = $state<Slot<Session[]>>(loading);
	let res = $state<Slot<Resource[]>>(loading);
	let pools = $state<Slot<Gateway[]>>(loading);
	let events = $state<Slot<AuditEvent[]>>(loading);
	let keys = $state<Slot<SSHKey[]>>(loading);
	let terminals = $state<Slot<AccessibleResource[]>>(loading);

	onMount(() => {
		const list = (status: SessionStatus, pageSize: number) => () => sessions.listSessions({ status, pageSize }).then((r) => r.sessions);
		load(list(SessionStatus.ACTIVE, ACTIVE_PAGE), (s) => (active = s));
		load(list(SessionStatus.UNSPECIFIED, 5), (s) => (recent = s));
		load(
			() => resources.listAccessibleResources({}).then((r) => r.resources.filter((x) => x.allowed && x.kinds.includes('terminal'))),
			(s) => (terminals = s)
		);
		if (manager) {
			load(() => resources.listResources({}).then((r) => r.resources), (s) => (res = s));
			load(() => gateways.listGateways({}).then((r) => r.gateways), (s) => (pools = s));
			load(() => audit.listEvents({ pageSize: 5 }).then((r) => r.events), (s) => (events = s));
		} else {
			load(() => sshKeys.listSSHKeys({}).then((r) => r.keys), (s) => (keys = s));
		}
	});

	// a failed call shows '—' instead of a number
	const counters = $derived.by(() => {
		const num = <T,>(s: Slot<T>, f: (v: T) => string) => (s.state === 'ok' ? f(s.value) : '—');
		return manager
			? [
					{ loading: active.state === 'loading', value: num(active, (v) => activeLabel(v.length)), label: m.dashboard_active_sessions() },
					{ loading: res.state === 'loading', value: num(res, (v) => String(v.length)), label: m.dashboard_resources() },
					{ loading: pools.state === 'loading', value: num(pools, (v) => `${onlineCount(v).online}/${v.length}`), label: m.dashboard_pools_online() }
				]
			: [
					{ loading: active.state === 'loading', value: num(active, (v) => activeLabel(v.length)), label: m.dashboard_active_sessions() },
					{ loading: keys.state === 'loading', value: num(keys, (v) => String(v.length)), label: m.dashboard_ssh_keys() }
				];
	});
</script>

<div class="scroll">
<header class="head">
	<p class="date">{date}</p>
	<h1 class="title">{data.session.organization?.name ?? m.dashboard_title()}</h1>
	<p class="version">{info.failed ? m.version_unavailable() : info.version && m.version_label({ version: info.version })}</p>
	<DashCounters {counters} />
</header>

<div class="grid">
	<SessionsCard {recent} />
	<TerminalsCard accessible={terminals} />
	{#if manager}
		<HealthCard resources={res} />
		<PoolsCard {pools} />
		<EventsCard {events} />
	{:else}
		<KeysCard {keys} />
	{/if}
</div>
</div>

<style>
	.scroll {
		padding: 32px 32px 48px;
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
</style>