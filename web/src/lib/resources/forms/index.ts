import type { Component } from 'svelte';
import { create } from '@bufbuild/protobuf';
import { anyPack, DurationSchema, type Any } from '@bufbuild/protobuf/wkt';
import { ConfigSchema } from '#lib/gen/werbot/connector/ssh/v1/ssh_pb.js';
import { ConfigSchema as TelnetConfigSchema } from '#lib/gen/werbot/connector/telnet/v1/telnet_pb.js';
import { ConfigSchema as TcpConfigSchema } from '#lib/gen/werbot/connector/tcp/v1/tcp_pb.js';
import { ConfigSchema as UdpConfigSchema } from '#lib/gen/werbot/connector/udp/v1/udp_pb.js';
import { ConfigSchema as PostgresConfigSchema } from '#lib/gen/werbot/connector/postgres/v1/postgres_pb.js';
import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
import SshConfigForm from './SshConfigForm.svelte';
import TelnetConfigForm from './TelnetConfigForm.svelte';
import TcpConfigForm from './TcpConfigForm.svelte';
import UdpConfigForm from './UdpConfigForm.svelte';
import PostgresConfigForm from './PostgresConfigForm.svelte';
import JsonConfigForm from './JsonConfigForm.svelte';
import { parsePorts } from './ports';

export type ConfigValue = { config?: Any; configJson?: string };
export type ConfigFormProps = { type: ResourceType; initial?: Resource; onchange: (v: ConfigValue) => void };

export const configForms: Record<string, Component<ConfigFormProps>> = {
	ssh: SshConfigForm,
	telnet: TelnetConfigForm,
	tcp: TcpConfigForm,
	udp: UdpConfigForm,
	postgres: PostgresConfigForm
};

export function configFormFor(type: string): Component<ConfigFormProps> {
	return configForms[type] ?? JsonConfigForm;
}

export function sshConfigValue(v: { host: string; port: number; login: string; portForwarding: boolean; remoteForwarding: boolean }): ConfigValue {
	return { config: anyPack(ConfigSchema, create(ConfigSchema, v)) };
}

export function telnetConfigValue(v: { host: string; port: number; login: string }): ConfigValue {
	return { config: anyPack(TelnetConfigSchema, create(TelnetConfigSchema, v)) };
}

// Port texts are parsed here and throw when the server would refuse them; the forms treat that as an invalid value.
function requiredPorts(text: string) {
	const ports = parsePorts(text);
	if (!ports.length) throw new Error('ports required');
	return ports;
}

export function tcpConfigValue(v: { host: string; ports: string }): ConfigValue {
	return { config: anyPack(TcpConfigSchema, create(TcpConfigSchema, { host: v.host, ports: requiredPorts(v.ports) })) };
}

// flowIdle is whole seconds, empty for the gateway default; anything else throws.
export function udpConfigValue(v: { host: string; ports: string; flowIdle: string }): ConfigValue {
	if (v.flowIdle && !/^\d+$/.test(v.flowIdle)) throw new Error('flow idle must be whole seconds');
	const flowIdle = v.flowIdle ? create(DurationSchema, { seconds: BigInt(v.flowIdle) }) : undefined;
	return { config: anyPack(UdpConfigSchema, create(UdpConfigSchema, { host: v.host, ports: requiredPorts(v.ports), flowIdle })) };
}

export const sslmodes = ['verify-full', 'require', 'disable'];

export type PostgresFields = {
	host: string;
	port: string;
	database: string;
	/** comma separated path.Match globs; empty allows any database */
	allowedDatabases: string;
	sslmode: string;
	caPem: string;
	recordParams: boolean;
	username: string;
	/** fields the form does not show; an edit keeps them */
	keep?: { secretTables: string[]; agentUsername: string };
};

// Throws when the server would refuse the port or sslmode; the form treats that as an invalid value.
export function postgresConfigValue(v: PostgresFields): ConfigValue {
	const port = v.port ? Number(v.port) : 0;
	if (!Number.isInteger(port) || port < 0 || port > 65535) throw new Error('port out of range');
	if (!sslmodes.includes(v.sslmode)) throw new Error('unknown sslmode');
	const allowedDatabases = v.allowedDatabases.split(',').map((s) => s.trim()).filter(Boolean);
	const { host, database, sslmode, caPem, recordParams, username } = v;
	return { config: anyPack(PostgresConfigSchema, create(PostgresConfigSchema, { host, port, database, allowedDatabases, sslmode, caPem, recordParams, username, ...v.keep })) };
}
