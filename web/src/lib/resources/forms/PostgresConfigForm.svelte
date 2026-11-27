<script lang="ts">
	import { anyIs, anyUnpack } from '@bufbuild/protobuf/wkt';
	import { Banner, FilledField, Textarea, Toggle } from '#lib/ui/index.js';
	import { ConfigSchema } from '#lib/gen/werbot/connector/postgres/v1/postgres_pb.js';
	import { m } from '#lib/paraglide/messages.js';
	import FieldRow from '#lib/FieldRow.svelte';
	import FilledSelect from '#lib/FilledSelect.svelte';
	import { postgresConfigValue, type ConfigFormProps } from './index.js';

	let { initial, onchange }: ConfigFormProps = $props();

	// svelte-ignore state_referenced_locally
	const current = initial?.config && anyIs(initial.config, ConfigSchema) ? anyUnpack(initial.config, ConfigSchema) : undefined;
	let host = $state(current?.host ?? '');
	let port = $state(String(current?.port || 5432));
	let database = $state(current?.database ?? '');
	let allowedDatabases = $state(current?.allowedDatabases.join(', ') ?? '');
	let sslmode = $state(current?.sslmode || 'verify-full');
	let caPem = $state(current?.caPem ?? '');
	let recordParams = $state(current?.recordParams ?? true);
	let username = $state(current?.username ?? '');
	const keep = current && { secretTables: current.secretTables, agentUsername: current.agentUsername };

	const sslOptions = [
		{ value: 'verify-full', label: m.resources_postgres_ssl_verify_full() },
		{ value: 'require', label: m.resources_postgres_ssl_require() },
		{ value: 'disable', label: m.resources_postgres_ssl_disable() }
	];

	$effect(() => {
		try {
			onchange(postgresConfigValue({ host, port, database, allowedDatabases, sslmode, caPem, recordParams, username, keep }));
		} catch {
			onchange({});
		}
	});
</script>

<FieldRow>
	<FilledField label={m.resources_ssh_host()} bind:value={host} required />
	<FilledField label={m.resources_ssh_port()} bind:value={port} type="number" min={1} max={65535} required />
</FieldRow>
<FieldRow even>
	<FilledField label={m.resources_postgres_username()} bind:value={username} maxlength={63} required />
	<FilledField label={m.resources_postgres_database()} bind:value={database} />
</FieldRow>
<FilledField label={m.resources_postgres_allowed_databases()} bind:value={allowedDatabases} placeholder="app, reports_*" />
<FilledSelect label={m.resources_postgres_sslmode()} options={sslOptions} bind:value={sslmode} />
{#if sslmode === 'disable'}
	<Banner tone="warning">{m.resources_postgres_ssl_disable_warning()}</Banner>
{/if}
{#if sslmode !== 'disable'}
	<Textarea label={m.resources_postgres_ca_pem()} bind:value={caPem} rows={5} spellcheck="false" placeholder="-----BEGIN CERTIFICATE-----" />
{/if}
<Toggle bind:checked={recordParams}>{m.resources_postgres_record_params()}</Toggle>
