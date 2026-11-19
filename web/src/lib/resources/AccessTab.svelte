<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner, Button, FilledField, Table, toast, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import { access, members, teams } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { Binding } from '#lib/gen/werbot/v1/access_pb.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FormField from '#lib/FormField.svelte';
	import CheckAccess from './CheckAccess.svelte';
	import { formatPorts, parsePorts } from './forms/ports.js';

	let { resource }: { resource: Resource } = $props();

	let bindings = $state<Binding[]>([]);
	let users = $state<SelectOption[]>([]);
	let teamOptions = $state<SelectOption[]>([]);
	let error = $state('');
	let subjectType = $state('user');
	let subjectId = $state('');
	let target = $state('resource');
	let selector = $state('');
	let ports = $state('');

	// ports only limit stream and datagram sessions, which only tcp and udp resources open
	const withPorts = $derived(resource.type === 'tcp' || resource.type === 'udp');
	const subjects = $derived(subjectType === 'user' ? users : teamOptions);
	const names = $derived(new Map([...users, ...teamOptions].map((o) => [o.value, o.label])));
	const targets = $derived([
		{ value: 'resource', label: m.access_this_resource() },
		{ value: 'selector', label: m.access_selector() }
	]);
	// ListBindings also returns every selector binding; show only those matching this resource.
	const visible = $derived(
		bindings.filter((b) => b.resourceId || Object.entries(b.labelSelector).every(([k, v]) => resource.labels[k] === v))
	);

	async function refresh() {
		({ bindings } = await access.listBindings({ resourceId: resource.id }));
	}

	onMount(async () => {
		try {
			const [, mem, tm] = await Promise.all([refresh(), members.listMembers({}), teams.listTeams({})]);
			users = mem.members.flatMap((x) => (x.user ? [{ value: x.user.id, label: `${x.user.name} (${x.user.login})` }] : []));
			teamOptions = tm.teams.map((t) => ({ value: t.id, label: t.name }));
		} catch (e) {
			error = errorMessage(e);
		}
	});

	async function run(action: () => Promise<unknown>) {
		error = '';
		try {
			await action();
			await refresh();
			toast({ tone: 'success', text: m.common_saved() });
		} catch (e) {
			error = errorMessage(e);
		}
	}

	function add(e: SubmitEvent) {
		e.preventDefault();
		const [key, ...rest] = selector.split('=');
		if (target === 'selector' && (!key || rest.length === 0)) {
			error = m.access_selector_invalid();
			return;
		}
		let portRanges;
		try {
			portRanges = withPorts ? parsePorts(ports) : [];
		} catch {
			error = m.resources_ports_invalid();
			return;
		}
		return run(() =>
			access.createBinding({
				subjectType,
				subjectId,
				ports: portRanges,
				...(target === 'resource' ? { resourceId: resource.id } : { labelSelector: { [key]: rest.join('=') } })
			})
		);
	}

	const describe = (b: Binding) =>
		b.resourceId ? m.access_this_resource() : Object.entries(b.labelSelector).map(([k, v]) => `${k}=${v}`).join(', ');
</script>

{#if error}<Banner tone="alert">{error}</Banner>{/if}
<h3>{m.access_bindings()}</h3>
{#if visible.length === 0}
	<p>{m.access_none()}</p>
{:else}
	<Table>
		{#snippet header()}
			<th>{m.access_subject()}</th>
			<th>{m.access_target()}</th>
			{#if withPorts}<th>{m.access_col_ports()}</th>{/if}
			<th></th>
		{/snippet}
		{#each visible as b (b.id)}
			<tr>
				<td>{names.get(b.subjectId) ?? b.subjectId}</td>
				<td>{describe(b)}</td>
				{#if withPorts}<td>{formatPorts(b.ports) || m.access_all_ports()}</td>{/if}
				<td><Button scheme="w" variant="plain" onclick={() => run(() => access.deleteBinding({ id: b.id }))}>{m.access_remove()}</Button></td>
			</tr>
		{/each}
	</Table>
{/if}

<form onsubmit={add}>
	<FilledSelect
		label={m.access_subject_type()}
		options={[{ value: 'user', label: m.access_user() }, { value: 'team', label: m.access_team() }]}
		bind:value={subjectType}
		onchange={() => (subjectId = '')}
	/>
	<FilledSelect label={m.access_subject()} options={subjects} bind:value={subjectId} />
	<FilledSelect label={m.access_target()} options={targets} bind:value={target} />
	{#if target === 'selector'}<FilledField label={m.access_selector()} bind:value={selector} placeholder="env=prod" />{/if}
	{#if withPorts}<FormField label={m.access_ports()} bind:value={ports} placeholder="22, 8000-8100" hint={m.access_ports_hint()} />{/if}
	<div><Button size={40} type="submit" variant="solid" disabled={!subjectId}>{m.access_add()}</Button></div>
</form>

<CheckAccess resourceId={resource.id} {users} {withPorts} />

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		max-width: 480px;
		margin-block: 16px;
	}
</style>
