<script lang="ts">
	import { ResourceHealth_State, type ResourceHealth } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';

	let { health }: { health?: ResourceHealth } = $props();

	const state = $derived(
		health?.state === ResourceHealth_State.HEALTHY ? 'healthy' : health?.state === ResourceHealth_State.UNHEALTHY ? 'unhealthy' : 'unknown'
	);
	const label = $derived({ healthy: m.resources_health_healthy(), unhealthy: m.resources_health_unhealthy(), unknown: m.resources_health_unknown() }[state]);
</script>

<span class="dot" data-health={state} title={health?.error || label} role="img" aria-label={label}></span>

<style>
	.dot {
		display: inline-block;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--miso-sidebar-badge-ghost);
	}

	.dot[data-health='healthy'] {
		background: var(--miso-hlt-invert);
	}

	.dot[data-health='unhealthy'] {
		background: var(--miso-sidebar-badge-alert);
	}
</style>
