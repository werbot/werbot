<script lang="ts">
	import FormField from '#lib/FormField.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { parsePorts } from './ports.js';

	let { value = $bindable(''), label, required = false }: { value?: string; label: string; required?: boolean } = $props();

	let touched = $state(false);

	// a pristine empty field shows no error
	const error = $derived.by(() => {
		if (!value && !touched) return '';
		try {
			return required && !parsePorts(value).length ? m.resources_ports_invalid() : '';
		} catch {
			return m.resources_ports_invalid();
		}
	});
</script>

<FormField {label} bind:value {required} {error} onblur={() => (touched = true)} placeholder="22, 8000-8100" />
