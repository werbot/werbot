<script lang="ts">
	import { Banner, Drawer, PropertyRow } from '#lib/ui/index.js';
	import { audit } from '#lib/api.js';
	import { errorMessage } from '#lib/errors.js';
	import type { AuditEvent } from '#lib/gen/werbot/v1/audit_pb.js';
	import { fmt } from '#lib/gateways/format.js';
	import { m } from '#lib/paraglide/messages.js';

	// id of the event to show; the drawer closes by clearing it
	let { id = $bindable() }: { id: string } = $props();

	let event = $state<AuditEvent>();
	let error = $state('');

	$effect(() => {
		if (!id) return;
		let stale = false;
		event = undefined;
		error = '';
		audit.getEvent({ id }).then(
			(r) => {
				if (!stale) event = r.event;
			},
			(e) => {
				if (!stale) error = errorMessage(e);
			}
		);
		return () => (stale = true);
	});
</script>

<Drawer open={!!id} title={m.audit_event_title()} onclose={() => (id = '')}>
	{#if error}<Banner tone="alert">{error}</Banner>{/if}
	{#if event}
		<PropertyRow label={m.audit_col_time()}>{fmt(event.createdAt)}</PropertyRow>
		<PropertyRow label={m.audit_col_actor()}>{event.actorLogin || event.actorKind}</PropertyRow>
		<PropertyRow label={m.audit_col_action()}>{event.action}</PropertyRow>
		<PropertyRow label={m.audit_col_target()}>{event.targetType} {event.targetId}</PropertyRow>
		<PropertyRow label={m.audit_ip()}>{event.ip || '—'}</PropertyRow>
		<PropertyRow label={m.audit_data()}><pre>{JSON.stringify(event.data ?? {}, null, 2)}</pre></PropertyRow>
	{/if}
</Drawer>

<style>
	pre {
		margin: 0;
		white-space: pre-wrap;
		word-break: break-all;
	}
</style>
