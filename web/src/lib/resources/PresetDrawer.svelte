<script lang="ts">
	import { untrack } from 'svelte';
	import { Button, Drawer, FilledField, toast } from '#lib/ui/index.js';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import FormField from '#lib/FormField.svelte';
	import FieldRow from '#lib/FieldRow.svelte';
	import { presets } from '#lib/api.js';
	import { failToast, serverText } from '#lib/errors.js';
	import type { ResourcePreset } from '#lib/gen/werbot/v1/preset_pb.js';
	import { selectOpen } from '#lib/selectOpen.js';
	import { m } from '#lib/paraglide/messages.js';
	import PortRangesField from './forms/PortRangesField.svelte';
	import { formatPorts, parsePorts } from './forms/ports.js';
	import { categoryLabel } from './icons.js';

	interface Props {
		open: boolean;
		/** set: edit this preset (name and base type stay); unset: create */
		preset?: ResourcePreset;
		onsaved: () => void;
	}

	let { open = $bindable(), preset, onsaved }: Props = $props();

	const categories = ['servers', 'databases', 'applications', 'desktops', 'containers', 'clouds'];
	const baseTypes = ['tcp', 'udp'].map((v) => ({ value: v, label: v }));
	const blank = { name: '', baseType: 'tcp', displayName: '', ports: '', category: 'databases', icon: '', launchHint: '' };
	let form = $state({ ...blank });

	// the form is filled each time the drawer opens
	$effect(() => {
		if (!open) return;
		untrack(() => {
			form = preset
				? { name: preset.name, baseType: preset.baseType, displayName: preset.displayName, ports: formatPorts(preset.defaultPorts), category: preset.category, icon: preset.icon, launchHint: preset.launchHint }
				: { ...blank };
		});
	});

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		try {
			const { ports, ...rest } = form;
			const req = { ...rest, defaultPorts: parsePorts(ports) };
			if (preset) await presets.updateResourcePreset({ ...req, id: preset.id });
			else await presets.createResourcePreset(req);
			toast({ tone: 'success', text: m.common_saved() });
			open = false;
			onsaved();
		} catch (err) {
			failToast(serverText(err));
		}
	}
</script>

<Drawer bind:open={() => open, (v) => !v && !selectOpen() && (open = false)} resizable snapPoints={[480, 720]} width={560} title={preset ? m.presets_edit() : m.presets_create()}>
	<form id="preset-form" onsubmit={submit}>
		<FieldRow even>
			<FormField label={m.presets_name_label()} hint={preset ? undefined : m.presets_name_hint()} bind:value={form.name} disabled={!!preset} required maxlength={32} />
			{#if preset}
				<FilledField label={m.presets_base_type_label()} value={form.baseType} disabled />
			{:else}
				<FilledSelect label={m.presets_base_type_label()} options={baseTypes} bind:value={form.baseType} />
			{/if}
		</FieldRow>
		<FieldRow even>
			<FormField label={m.presets_display_name_label()} bind:value={form.displayName} required maxlength={64} />
			<FilledSelect label={m.presets_category_label()} options={categories.map((c) => ({ value: c, label: categoryLabel(c) }))} bind:value={form.category} />
		</FieldRow>
		<FieldRow even>
			<PortRangesField label={m.presets_ports_label()} bind:value={form.ports} required />
			<FormField label={m.presets_icon_label()} hint={m.presets_icon_hint()} bind:value={form.icon} maxlength={64} />
		</FieldRow>
		<FormField label={m.presets_launch_hint_label()} hint={m.presets_launch_hint_help({ host: '{host}', port: '{port}' })} placeholder={'psql -h {host} -p {port}'} bind:value={form.launchHint} maxlength={512} />
	</form>
	{#snippet footer()}
		<Button size={40} scheme="g" variant="plain" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button size={40} type="submit" form="preset-form" variant="solid">{m.resources_save()}</Button>
	{/snippet}
</Drawer>

<style>
	form {
		display: flex;
		flex-direction: column;
		gap: var(--miso-sp-12);
	}
</style>
