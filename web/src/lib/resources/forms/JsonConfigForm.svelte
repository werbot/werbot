<script lang="ts">
	import { Textarea } from '#lib/ui/index.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { ConfigFormProps } from './index.js';

	let { initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	let text = $state(initial?.configJson || '{}');
	let error = $state('');

	$effect(() => {
		try {
			JSON.parse(text);
		} catch {
			error = m.resources_config_json_invalid();
			return;
		}
		error = '';
		onchange({ configJson: text });
	});
</script>

<Textarea label={m.resources_config_json()} bind:value={text} {error} rows={8} />
