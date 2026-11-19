<script lang="ts">
	import { Button, Segmented, FilledField } from '#lib/ui/index.js';
	import { system } from '#lib/api.js';
	import { m } from '#lib/paraglide/messages.js';
	import type { NetworkRule } from '#lib/gen/werbot/v1/policy_pb.js';
	import { effectOptions } from './effects.js';

	let { rows = $bindable(), onerror }: { rows: NetworkRule[]; onerror: (e: unknown) => void } = $props();

	async function useMyIP() {
		try {
			const { ip } = await system.getClientIP({});
			rows.push({ $typeName: 'werbot.v1.NetworkRule', cidr: ip + (ip.includes(':') ? '/128' : '/32'), effect: 'allow' });
		} catch (e) {
			onerror(e);
		}
	}
</script>

<fieldset>
	<legend>{m.rules_networks()}</legend>
	{#each rows as row, i (i)}
		<div class="row">
			<FilledField compact label={m.rules_cidr()} bind:value={row.cidr} />
			<Segmented options={effectOptions()} bind:value={row.effect} />
			<Button type="button" scheme="w" variant="plain" onclick={() => rows.splice(i, 1)}>{m.rules_remove()}</Button>
		</div>
	{/each}
	<div class="wb-actions">
		<Button type="button" variant="plain" onclick={() => rows.push({ $typeName: 'werbot.v1.NetworkRule', cidr: '', effect: 'allow' })}>
			{m.rules_add_network()}
		</Button>
		<Button type="button" variant="plain" onclick={useMyIP}>{m.rules_my_ip()}</Button>
	</div>
</fieldset>

<style>
	fieldset {
		display: flex;
		flex-direction: column;
		gap: 8px;
		border: 0;
		padding: 0;
	}

	.row {
		display: flex;
		gap: 8px;
		align-items: center;
	}
</style>
