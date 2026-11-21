<script lang="ts">
	import { Banner, Button, Checkbox, Drawer, toast, type SelectOption } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FormField from '#lib/FormField.svelte';
	import { accessRequests, resources } from '#lib/api.js';
	import { failToast } from '#lib/errors.js';
	import { parseDuration } from '#lib/duration.js';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		open: boolean;
		/** the new request's id: the page opens it */
		oncreated: (id: string) => void;
	}

	let { open = $bindable(), oncreated }: Props = $props();

	const blank = { resourceId: '', duration: '', reason: '', breakGlass: false };
	let form = $state({ ...blank });
	let options = $state<SelectOption[] | null>(null);
	let durationError = $state('');

	async function loadOptions() {
		try {
			const { resources: list } = await resources.listAccessibleResources({});
			options = list.filter((r) => r.requestable).map((r) => ({ value: r.id, label: r.name }));
		} catch (e) {
			failToast(e);
			options = [];
		}
	}

	// the requestable list is read each time the drawer opens
	$effect(() => {
		if (open) loadOptions();
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		const duration = form.duration.trim() ? parseDuration(form.duration) : undefined;
		durationError = duration === null ? m.common_duration_invalid() : '';
		if (duration === null) return;
		try {
			const { request } = await accessRequests.createRequest({ resourceId: form.resourceId, duration, reason: form.reason, breakGlass: form.breakGlass });
			toast({ tone: 'success', text: m.requests_sent() });
			form = { ...blank };
			open = false;
			oncreated(request?.id ?? '');
		} catch (err) {
			failToast(err);
		}
	}
</script>

<Drawer bind:open={() => open, (v) => !v && !selectOpen() && (open = false)} resizable snapPoints={[480, 720]} width={480} title={m.requests_new()}>
	<form id="new-request" onsubmit={submit}>
		<FilledSelect label={m.requests_resource()} options={options ?? []} bind:value={form.resourceId} />
		{#if options?.length === 0}<p class="note">{m.requests_none_requestable()}</p>{/if}
		<FormField label={m.requests_duration()} placeholder="2h" hint={m.requests_duration_hint()} error={durationError} bind:value={form.duration} />
		<FormField multiline label={m.requests_reason()} bind:value={form.reason} maxlength={1024} />
		<Checkbox bind:checked={form.breakGlass}>{m.requests_break_glass()}</Checkbox>
		{#if form.breakGlass}<Banner tone="warning">{m.requests_break_glass_warning()}</Banner>{/if}
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="new-request" variant="solid" disabled={!form.resourceId}>{m.requests_submit()}</Button>
	{/snippet}
</Drawer>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}

	.note {
		margin: 0;
		font: var(--miso-font-ui-12-reg);
		color: var(--miso-txt-tertiary);
	}
</style>
