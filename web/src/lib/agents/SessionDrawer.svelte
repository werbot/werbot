<script lang="ts">
	import { Drawer, PropertyRow } from '#lib/ui/index.js';
	import type { RunSession } from '#lib/gen/werbot/v1/agent_pb.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import CastPlayer from '#lib/player/CastPlayer.svelte';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** the session to replay; null closes the drawer */
		session: RunSession | null;
		onclose: () => void;
	}

	let { session, onclose }: Props = $props();

	const shown = keepLast(() => session);
	const hex = (b: Uint8Array) => Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('');
</script>

<Drawer bind:open={() => !!session, (v) => !v && onclose()} resizable snapPoints={[640, 880]} width={640} title={shown.value?.resourceName ?? ''}>
	{#if shown.value}
		{@const s = shown.value}
		{#key s.id}
			<CastPlayer src={`/api/recordings/${s.id}.cast`} onverified={() => {}} />
		{/key}
		<PropertyRow label={m.agents_timeline_replay()}><code>{s.command}</code></PropertyRow>
		{#if s.classes.length > 0}<PropertyRow label={m.policies_commands_classes()}><ClassChips classes={s.classes} /></PropertyRow>{/if}
		<PropertyRow label={m.agents_timeline_exit_code()}>{s.exitCode ?? '—'}</PropertyRow>
		{#if s.outputSha256.length > 0}<PropertyRow label={m.agents_timeline_output_sha256()}><code>{hex(s.outputSha256)}</code></PropertyRow>{/if}
	{/if}
</Drawer>

<style>
	code {
		font-family: var(--miso-font-mono);
		font-size: 12px;
		overflow-wrap: anywhere;
	}
</style>
