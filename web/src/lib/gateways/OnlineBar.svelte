<script lang="ts">
	import { ProgressBar } from '#lib/ui/index.js';
	import { onlineCount } from '#lib/dashboard/dashboard.js';
	import { m } from '#lib/paraglide/messages.js';

	let { instances }: { instances: { online: boolean }[] } = $props();

	const count = $derived(onlineCount(instances));
</script>

<span class="inst" title={m.gateways_instances_title()}>
	<span class="inst__bar"><ProgressBar value={count.total ? count.online / count.total : 0} thickness={4} /></span>
	{count.online}/{count.total}
</span>

<style>
	.inst {
		display: inline-flex;
		align-items: center;
		gap: var(--miso-sp-06);
		white-space: nowrap;
	}
	.inst__bar {
		width: 48px;
	}
</style>
