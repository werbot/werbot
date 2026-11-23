<script lang="ts">
	import { Badge, EmptyState, Icon, Label, type IconName } from '#lib/ui/index.js';
	import ClassChips from '#lib/policies/ClassChips.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { TimelineItem, TimelineKind } from './timeline.js';

	interface Props {
		items: TimelineItem[];
		/** a session row was clicked */
		onsession: (id: string) => void;
	}

	let { items, onsession }: Props = $props();

	const icons: Record<TimelineKind, IconName> = { tool_call: 'gear', session: 'desktop', approval: 'front_hand', denial: 'shield' };
	const kinds: Record<TimelineKind, () => string> = {
		tool_call: m.agents_timeline_tool_call,
		session: m.agents_timeline_session,
		approval: m.agents_timeline_approval,
		denial: m.agents_timeline_denial
	};
</script>

{#if items.length === 0}
	<EmptyState icon="history" title={m.agents_timeline_empty()} />
{:else}
	<ol>
		{#each items as i (i.kind + i.id)}
			<li>
				<span class="tile"><Icon name={icons[i.kind]} size={16} /></span>
				<div class="body">
					<div class="head">
						<span class="kind">{kinds[i.kind]()}</span>
						<span class="muted">{i.at.toLocaleString()}</span>
						{#if i.status}<Badge tone={i.kind === 'denial' || i.status === 'denied' ? 'alert' : 'ghost'}>{i.status}</Badge>{/if}
						{#if i.kind === 'tool_call'}<Label variant="soft">{m.agents_timeline_client_reported()}</Label>{/if}
					</div>
					{#if i.sessionId}
						<button class="row-name title" onclick={() => onsession(i.sessionId!)}>{i.title}</button>
					{:else}
						<code class="title">{i.title}</code>
					{/if}
					{#if i.classes.length > 0}<ClassChips classes={i.classes} />{/if}
				</div>
			</li>
		{/each}
	</ol>
{/if}

<style>
	ol {
		list-style: none;
		margin: 0;
		padding: 0;
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	li {
		display: flex;
		gap: var(--miso-sp-12);
	}

	.tile {
		flex: none;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 32px;
		height: 32px;
		border-radius: var(--miso-radius-08);
		background: var(--miso-cont-top);
		color: var(--miso-txt-secondary);
	}

	.body {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-04);
		min-width: 0;
	}

	.head {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: var(--miso-sp-08);
	}

	.kind {
		font-weight: 600;
	}

	.muted {
		color: var(--miso-txt-secondary);
		white-space: nowrap;
	}

	.title {
		font-family: var(--miso-font-mono);
		font-size: 12px;
		overflow-wrap: anywhere;
		text-align: left;
	}
</style>
