<script lang="ts">
	import { Badge, type BadgeTone } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	let { status }: { status: string } = $props();

	const tones: Record<string, BadgeTone> = { running: 'accent', paused: 'upcoming', ended: 'ghost' };
	const labels: Record<string, () => string> = {
		running: m.agents_run_status_running,
		paused: m.agents_run_status_paused,
		ended: m.agents_run_status_ended
	};
</script>

<Badge tone={tones[status] ?? 'ghost'}>{labels[status]?.() ?? status}</Badge>
