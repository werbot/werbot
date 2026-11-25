<script lang="ts" module>
	export interface KillTarget {
		type: 'agent' | 'agent_run' | 'agents_of_user' | 'all_agents';
		/** empty for all_agents */
		id: string;
		/** drawer title: what the lock stops */
		title: string;
	}
</script>

<script lang="ts">
	import { Button, Drawer, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { locks } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { keepLast } from '#lib/keepLast.svelte.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** what to lock; null closes the drawer */
		target: KillTarget | null;
		onclose: () => void;
	}

	let { target, onclose }: Props = $props();

	const shown = keepLast(() => target);
	let reason = $state('');

	async function lock(e: SubmitEvent) {
		e.preventDefault();
		if (!target) return;
		try {
			await locks.createLock({ targetType: target.type, targetId: target.id, reason });
			toast({ tone: 'success', text: m.locks_created() });
			reason = '';
			onclose();
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Drawer bind:open={() => !!target, (v) => !v && onclose()} width={480} title={shown.value?.title ?? ''}>
	<form id="kill-switch" onsubmit={lock}>
		<FormField multiline label={m.agents_kill_reason()} bind:value={reason} required maxlength={1024} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={onclose}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="kill-switch" scheme="w" variant="solid" disabled={!reason.trim()}>{m.agents_kill_confirm()}</Button>
	{/snippet}
</Drawer>
