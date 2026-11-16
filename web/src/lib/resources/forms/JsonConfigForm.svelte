<script lang="ts">
	import { Textarea } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { ConfigFormProps } from './index.js';

	let { initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	let text = $state(initial?.configJson || '{}');
	let valid = $derived.by(() => {
		try {
			JSON.parse(text);
			return true;
		} catch {
			return false;
		}
	});

	// An empty value blocks submit in ResourceForm while the JSON is invalid.
	$effect(() => onchange(valid ? { configJson: text } : {}));
</script>

<Textarea label={m.resources_config_json()} bind:value={text} error={valid ? '' : m.resources_config_json_invalid()} rows={8} />
