<script lang="ts">
	import { Badge, Banner, Button, FilledField } from '#lib/ui/index.js';
	import { resources } from '#lib/api.js';
	import type { Resource } from '#lib/gen/werbot/v1/resource_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import { AGENT_SLOT, agentStatus } from './agentCredential.js';

	interface Props {
		resource: Resource;
		/** runs a credential change, reloads the resource; false when it failed (already toasted) */
		run: (action: () => Promise<unknown>) => Promise<boolean>;
	}

	let { resource, run }: Props = $props();

	let value = $state('');
	const status = $derived(agentStatus(resource.agentReadonlyStatus));

	// the example of docs/databases.md: a login role that can only read
	const roleSQL = `CREATE ROLE werbot_agent LOGIN PASSWORD '…';
GRANT CONNECT ON DATABASE app TO werbot_agent;
GRANT USAGE ON SCHEMA public TO werbot_agent;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO werbot_agent;`;

	async function save(e: SubmitEvent) {
		e.preventDefault();
		const bytes = new TextEncoder().encode(value);
		if (await run(() => resources.setCredential({ resourceId: resource.id, kind: 'password', value: bytes, slot: AGENT_SLOT }))) value = '';
	}
</script>

<section aria-labelledby="agent-credential">
	<h3 id="agent-credential">{m.resources_agent_credential()}</h3>
	<p class="status">
		<Badge tone={status.tone}>{status.text}</Badge>
		{#if resource.agentReadonlyDetail}<span class="detail">{resource.agentReadonlyDetail}</span>{/if}
	</p>
	<Banner tone="info">
		<span>{m.resources_agent_readonly_hint()}</span>
		<pre>{roleSQL}</pre>
	</Banner>
	<form onsubmit={save}>
		<FilledField type="password" label={m.cred_kind_password()} bind:value autocomplete="new-password" required />
		<div class="wb-actions">
			<Button size={40} type="submit" variant="solid">{m.cred_save()}</Button>
			<Button
				size={40}
				type="button"
				scheme="w"
				variant="plain"
				disabled={!resource.agentCredentialKind}
				onclick={() => run(() => resources.clearCredential({ resourceId: resource.id, slot: AGENT_SLOT }))}
			>
				{m.cred_clear()}
			</Button>
		</div>
	</form>
</section>

<style>
	section,
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
		max-width: 480px;
	}

	section {
		margin-top: var(--miso-sp-24);
	}

	h3 {
		margin: 0;
		font: var(--miso-font-ui-14-semi);
	}

	.status {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--miso-sp-08);
		margin: 0;
	}

	.detail {
		color: var(--miso-txt-secondary);
		font: var(--miso-font-ui-12-reg);
		overflow-wrap: anywhere;
	}

	pre {
		margin: var(--miso-sp-08) 0 0;
		font: var(--miso-font-ui-12-reg);
		font-family: var(--miso-font-mono);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
