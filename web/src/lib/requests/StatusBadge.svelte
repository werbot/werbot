<script lang="ts">
	import { Badge, type BadgeTone } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';

	let { status }: { status: string } = $props();

	const tones: Record<string, BadgeTone> = { pending: 'upcoming', approved: 'accent', denied: 'alert' };
	const labels: Record<string, () => string> = {
		pending: m.requests_status_pending,
		approved: m.requests_status_approved,
		denied: m.requests_status_denied,
		expired: m.requests_status_expired,
		canceled: m.requests_status_canceled
	};
</script>

<Badge tone={tones[status] ?? 'ghost'}>{labels[status]?.() ?? status}</Badge>
