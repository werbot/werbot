<script lang="ts">
	import { Chip } from '#lib/ui/index.js';
	import type { ResourceSelector, Subjects } from '#lib/gen/werbot/v1/selector_pb.js';
	import LabelChips from '#lib/resources/LabelChips.svelte';
	import { m } from '#lib/paraglide/messages.js';

	/** the "who" and "resources" table cells of a rule; `names` maps ids to names */
	let { subjects, selector, names }: { subjects?: Subjects; selector?: ResourceSelector; names: ReadonlyMap<string, string> } = $props();

	const who = $derived([...(subjects?.users ?? []), ...(subjects?.teams ?? [])]);
	const ids = $derived(selector?.resourceIds ?? []);
</script>

<td>
	<div class="chips">
		{#if subjects?.allUsers}<Chip size={20}>{m.policies_commands_all_users()}</Chip>{/if}
		{#if subjects?.allAgents}<Chip size={20}>{m.policies_commands_all_agents()}</Chip>{/if}
		{#each who as id (id)}<Chip size={20}>{names.get(id) ?? id}</Chip>{/each}
	</div>
</td>
<td>
	{#if ids.length > 0}
		<div class="chips">{#each ids as id (id)}<Chip size={20}>{names.get(id) ?? id}</Chip>{/each}</div>
	{:else}
		<LabelChips labels={selector?.labels ?? {}} />
	{/if}
</td>

<style>
	.chips {
		display: flex;
		flex-wrap: wrap;
		gap: var(--miso-sp-04);
	}
</style>
