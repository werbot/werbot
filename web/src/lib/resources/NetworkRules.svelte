<script lang="ts">
	import { Button, Chip } from '#lib/ui/index.js';
	import FormField from '#lib/FormField.svelte';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';
	import { cidrOf } from './cidr.js';

	/** The networks of one rule list as tags; a new one goes in through the field. */
	let { cidrs = $bindable(), onerror }: { cidrs: string[]; onerror: (e: unknown) => void } = $props();

	let draft = $state('');
	let error = $state('');

	// add puts value in the list once, in the form the server stores
	function add(value: string) {
		const cidr = cidrOf(value);
		if (!cidr) {
			error = m.rules_cidr_invalid();
			return;
		}
		if (!cidrs.includes(cidr)) cidrs.push(cidr);
		draft = error = '';
	}

	function onkeydown(event: KeyboardEvent) {
		if (event.key !== 'Enter') return;
		event.preventDefault();
		add(draft);
	}

	async function useMyIP() {
		try {
			const { ip } = await system.getClientIP({});
			add(ip);
		} catch (e) {
			onerror(e);
		}
	}
</script>

<div class="rules">
	{#if cidrs.length > 0}
		<div class="chips">
			{#each cidrs as cidr (cidr)}
				<Chip deletable ondelete={() => cidrs.splice(cidrs.indexOf(cidr), 1)}>{cidr}</Chip>
			{/each}
		</div>
	{/if}
	<div class="row">
		<div class="field">
			<FormField label={m.rules_cidr()} placeholder="10.0.0.0/8" bind:value={draft} {error} hint={m.rules_cidr_hint()} {onkeydown} oninput={() => (error = '')} />
		</div>
		<Button type="button" size={40} variant="plain" onclick={() => add(draft)}>{m.rules_add_network()}</Button>
		<Button type="button" size={40} variant="plain" onclick={useMyIP}>{m.rules_my_ip()}</Button>
	</div>
</div>

<style>
	.rules {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-08);
	}

	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-08);
	}

	/* the buttons line up with the field, above its note */
	.row {
		display: flex;
		gap: var(--miso-sp-08);
		align-items: flex-start;
	}

	.row > :global(.miso-btn) {
		margin-top: 10px;
	}

	.field {
		flex: 1;
		min-width: 0;
	}
</style>
