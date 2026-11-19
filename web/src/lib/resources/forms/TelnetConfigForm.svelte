<script lang="ts">
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { Banner, FilledField } from '#lib/ui/index.js';
	import { ConfigSchema } from '#lib/gen/werbot/connector/telnet/v1/telnet_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import { telnetConfigValue, type ConfigFormProps } from './index.js';

	let { initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	const current = initial?.config && anyIs(initial.config, ConfigSchema) ? anyUnpack(initial.config, ConfigSchema) : undefined;
	let host = $state(current?.host ?? '');
	let port = $state(String(current?.port ?? 23));
	let login = $state(current?.login ?? '');

	$effect(() => onchange(telnetConfigValue({ host, port: Number(port) || 0, login })));
</script>

<Banner tone="warning">{m.resources_telnet_plaintext()}</Banner>
<FieldRow>
	<FilledField label={m.resources_ssh_host()} bind:value={host} required />
	<FilledField label={m.resources_ssh_port()} bind:value={port} type="number" min={1} max={65535} required />
</FieldRow>
<FieldRow even>
	<FilledField label={m.resources_ssh_login()} bind:value={login} />
</FieldRow>
