<script lang="ts">
	import { TextField } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import { parsePorts } from './ports.js';

	let { value = $bindable(''), label, required = false }: { value?: string; label: string; required?: boolean } = $props();

	const error = $derived.by(() => {
		try {
			return required && !parsePorts(value).length ? m.resources_ports_invalid() : '';
		} catch {
			return m.resources_ports_invalid();
		}
	});
</script>

<TextField {label} bind:value {required} {error} placeholder="22, 8000-8100" />
