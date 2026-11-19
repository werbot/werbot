<script lang="ts">
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { FilledField, Toggle } from '#lib/ui/index.js';
	import { ConfigSchema } from '#lib/gen/werbot/connector/ssh/v1/ssh_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import { sshConfigValue, type ConfigFormProps } from './index.js';

	let { initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	const current = initial?.config && anyIs(initial.config, ConfigSchema) ? anyUnpack(initial.config, ConfigSchema) : undefined;
	let host = $state(current?.host ?? '');
	let port = $state(String(current?.port ?? 22));
	let login = $state(current?.login ?? '');
	let portForwarding = $state(current?.portForwarding ?? false);

	$effect(() => onchange(sshConfigValue({ host, port: Number(port) || 0, login, portForwarding })));
</script>

<FieldRow>
	<FilledField label={m.resources_ssh_host()} bind:value={host} required />
	<FilledField label={m.resources_ssh_port()} bind:value={port} type="number" min={1} max={65535} required />
</FieldRow>
<FieldRow even>
	<FilledField label={m.resources_ssh_login()} bind:value={login} required />
	<div class="toggle"><Toggle bind:checked={portForwarding}>{m.resources_ssh_port_forwarding()}</Toggle></div>
</FieldRow>

<style>
	/* centred against the login field next to it */
	.toggle {
		align-self: center;
	}
</style>
