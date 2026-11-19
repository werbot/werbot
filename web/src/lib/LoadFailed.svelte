<script lang="ts">
	import { Button, EmptyState } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	// a failed load: the toast says why, this keeps the content area from going blank
	let { onretry }: { onretry: () => void } = $props();
</script>

<EmptyState icon="cloud" title={m.common_load_failed()}>
	{#snippet actions()}
		<Button scheme="g" variant="plain" size={28} iconLeft="repeat" onclick={onretry}>{m.common_retry()}</Button>
	{/snippet}
</EmptyState>
