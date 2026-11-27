<script lang="ts">
	import { onMount } from 'svelte';
	import { Banner, Button, FilledField, Skeleton, Table, Toggle } from '#lib/ui/index.js';
	import { failToast } from '#lib/errors.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FieldRow from '#lib/FieldRow.svelte';
	import LoadFailed from '#lib/LoadFailed.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import {
		distinct,
		entryText,
		filterQueryLog,
		parseQueryLog,
		queryLogPage,
		verifiedBanner,
		QUERYLOG_PAGE,
		type QueryLogEntry
	} from './querylog.js';

	let { src }: { src: string } = $props();

	let entries = $state<QueryLogEntry[]>([]);
	let invalid = $state(0);
	let tone = $state<ReturnType<typeof verifiedBanner>>();
	let loading = $state(true);
	let failed = $state(false);

	let kind = $state('');
	let decision = $state('');
	let text = $state('');
	let errorsOnly = $state(false);
	let shown = $state(QUERYLOG_PAGE);
	let open = $state<number>();

	const filtered = $derived(filterQueryLog(entries, { kind, decision, text, errorsOnly }));
	const page = $derived(queryLogPage(filtered, shown));
	const choices = (key: 'kind' | 'decision') => [{ value: '', label: m.querylog_filter_all() }, ...distinct(entries, key).map((v) => ({ value: v, label: v }))];

	async function load() {
		failed = false;
		loading = true;
		try {
			const r = await fetch(src, { credentials: 'same-origin' });
			if (!r.ok) throw new Error(String(r.status));
			const body = await r.text(); // a broken body must not show "verified"
			({ entries, invalid } = parseQueryLog(body));
			tone = verifiedBanner(r.headers.get('X-Werbot-Recording-Verified'));
		} catch {
			failed = true;
			failToast(m.sessions_recording_load_error());
		} finally {
			loading = false;
		}
	}

	onMount(load);

	// a changed filter starts again from the first page
	function refilter(set: () => void) {
		set();
		shown = QUERYLOG_PAGE;
		open = undefined;
	}
</script>

{#if failed}
	<LoadFailed onretry={load} />
{:else if loading}
	<Skeleton variant="rect" height={240} />
{:else}
	{#if tone}
		<Banner tone={tone}>{tone === 'success' ? m.sessions_recording_verified() : m.sessions_recording_failed()}</Banner>
	{/if}
	{#if invalid > 0}<Banner tone="info">{m.querylog_invalid({ count: invalid })}</Banner>{/if}
	<div class="filters">
		<FieldRow even>
			<FilledSelect label={m.querylog_filter_kind()} options={choices('kind')} value={kind} onchange={(v) => refilter(() => (kind = v))} />
			<FilledSelect label={m.querylog_filter_decision()} options={choices('decision')} value={decision} onchange={(v) => refilter(() => (decision = v))} />
		</FieldRow>
		<FilledField label={m.querylog_filter_text()} value={text} oninput={(e) => refilter(() => (text = e.currentTarget.value))} />
		<Toggle checked={errorsOnly} onchange={(e) => refilter(() => (errorsOnly = e.currentTarget.checked))}>{m.querylog_errors_only()}</Toggle>
	</div>
	{#if page.visible.length === 0}
		<p>{m.querylog_empty()}</p>
	{:else}
		<div class="scroll">
			<Table>
				{#snippet header()}
					<th>{m.querylog_col_time()}</th>
					<th>{m.querylog_col_kind()}</th>
					<th>{m.querylog_col_text()}</th>
					<th>{m.querylog_col_db()}</th>
					<th>{m.querylog_col_rows()}</th>
					<th>{m.querylog_col_ms()}</th>
					<th>{m.querylog_col_decision()}</th>
					<th>{m.querylog_col_error()}</th>
				{/snippet}
				{#each page.visible as e, i (i)}
					<tr>
						<td>{e.t.toFixed(3)}</td>
						<td>{e.kind}</td>
						<td class="text">
							<button type="button" class={['sql', open !== i && 'sql--clamp']} aria-expanded={open === i} onclick={() => (open = open === i ? undefined : i)}>{entryText(e)}</button>
							{#if open === i}
								{#if e.statement}<div class="more"><code>{e.statement}</code></div>{/if}
								{#if e.params?.length}<div class="more">{m.querylog_params()}: <code>{e.params.join(', ')}</code></div>{/if}
								{#if e.tables?.length}<div class="more">{m.querylog_tables()}: <code>{e.tables.join(', ')}</code></div>{/if}
							{/if}
						</td>
						<td>{e.db ?? ''}</td>
						<td>{e.rows ?? ''}</td>
						<td>{e.duration_us === undefined ? '' : (e.duration_us / 1000).toFixed(2)}</td>
						<td>{e.decision ?? ''}</td>
						<td>{e.error ?? ''}</td>
					</tr>
				{/each}
			</Table>
		</div>
		{#if page.rest > 0}
			<div><Button size={40} scheme="g" variant="plain" onclick={() => (shown += QUERYLOG_PAGE)}>{m.querylog_show_more({ count: page.rest })}</Button></div>
		{/if}
	{/if}
{/if}

<style>
	.filters {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.scroll {
		overflow-x: auto;
	}

	.text {
		min-width: 240px;
		max-width: 480px;
	}

	.sql {
		all: unset;
		display: block;
		cursor: pointer;
		font-family: var(--miso-font-mono);
		font-size: 12px;
		overflow-wrap: anywhere;
		white-space: pre-wrap;
	}

	.sql--clamp {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}

	.sql:focus-visible {
		outline: 2px solid var(--miso-line-accent, currentColor);
	}

	.more {
		font-size: 12px;
		margin-top: var(--miso-sp-04, 4px);
		overflow-wrap: anywhere;
	}
</style>
