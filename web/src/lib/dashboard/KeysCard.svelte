<script lang="ts">
	import type { SSHKey } from '#lib/gen/werbot/v1/ssh_key_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import Ago from '#lib/Ago.svelte';
	import DashCard from './DashCard.svelte';
	import DashRow from './DashRow.svelte';
	import type { Slot } from './dashboard';

	let { keys }: { keys: Slot<SSHKey[]> } = $props();
</script>

<DashCard
	title={m.dashboard_ssh_keys()}
	icon="key"
	href="/profile/keys"
	slot={keys}
	count={keys.state === 'ok' && keys.value.length ? String(keys.value.length) : undefined}
	empty={{ icon: 'key', title: m.dashboard_no_keys() }}
>
	{#snippet children(list)}
		<ul>
			{#each list.slice(0, 5) as k (k.id)}
				<DashRow href="/profile/keys" title={k.name} icon="key">
					{#snippet meta()}{k.fingerprintSha256}{/snippet}
					{#snippet trail()}<Ago ts={k.createdAt} />{/snippet}
				</DashRow>
			{/each}
		</ul>
	{/snippet}
</DashCard>
