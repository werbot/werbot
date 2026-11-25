<script lang="ts">
	import { Button, Drawer, toast } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { sessions } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		/** session to terminate; null closes the drawer */
		id: string | null;
		onclose: () => void;
	}

	let { id, onclose }: Props = $props();

	let reason = $state('');

	async function terminate(e: SubmitEvent) {
		e.preventDefault();
		if (!id) return;
		try {
			await sessions.terminateSession({ id, reason });
			toast({ tone: 'success', text: m.live_terminated() });
			reason = '';
			onclose();
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Drawer bind:open={() => !!id, (v) => !v && onclose()} width={480} title={m.live_terminate_title()}>
	<form id="terminate-session" onsubmit={terminate}>
		<FormField multiline label={m.live_terminate_reason()} bind:value={reason} required maxlength={256} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={onclose}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="terminate-session" scheme="w" variant="solid" disabled={!reason.trim()}>{m.live_terminate_confirm()}</Button>
	{/snippet}
</Drawer>
