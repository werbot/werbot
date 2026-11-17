import { anyUnpack } from '@bufbuild/protobuf/wkt';
import { expect, test } from 'vitest';
import { ConfigSchema as UdpConfigSchema } from '#lib/gen/werbot/connector/udp/v1/udp_pb.js';
import { ConfigSchema as TcpConfigSchema } from '#lib/gen/werbot/connector/tcp/v1/tcp_pb.js';
import { configForms, configFormFor, sshConfigValue, tcpConfigValue, telnetConfigValue, udpConfigValue } from './index';
import { parsePorts } from './ports';
import SshConfigForm from './SshConfigForm.svelte';
import JsonConfigForm from './JsonConfigForm.svelte';

test('ssh, telnet, tcp and udp have dedicated forms, unknown types fall back to JSON', () => {
	expect(Object.keys(configForms)).toEqual(['ssh', 'telnet', 'tcp', 'udp']);
	expect(configFormFor('ssh')).toBe(SshConfigForm);
	expect(configFormFor('echo')).toBe(JsonConfigForm);
});

test('ssh value is packed as Any of werbot.connector.ssh.v1.Config', () => {
	expect(sshConfigValue({ host: 'h', port: 22, login: 'root', portForwarding: true }).config?.typeUrl).toBe(
		'type.googleapis.com/werbot.connector.ssh.v1.Config'
	);
});

test('telnet value is packed with its type url', () => {
	expect(telnetConfigValue({ host: 'h', port: 23, login: '' }).config?.typeUrl).toBe(
		'type.googleapis.com/werbot.connector.telnet.v1.Config'
	);
});

test('tcp value carries the parsed port ranges', () => {
	const { config } = tcpConfigValue({ host: 'h', ports: '22, 8000-8100' });
	const c = anyUnpack(config!, TcpConfigSchema) as { ports: { port: number; endPort: number }[] };
	expect(c.ports.map((p) => [p.port, p.endPort])).toEqual([[22, 0], [8000, 8100]]);
});

test('udp flow idle is optional', () => {
	expect(udpConfigValue({ host: 'h', ports: '53', flowIdle: '' }).config?.typeUrl).toBe(
		'type.googleapis.com/werbot.connector.udp.v1.Config'
	);
});

test.each(['0', '10-5', '', '70000', '1-70000', 'a', '1-2-3', '10-20, 15', '1,1'])('ports %j are rejected', (text) => {
	expect(() => tcpConfigValue({ host: 'h', ports: text })).toThrow();
});

test('at most 16 ranges', () => {
	const ranges = (n: number) => Array.from({ length: n }, (_, i) => i + 1).join(',');
	expect(parsePorts(ranges(16))).toHaveLength(16);
	expect(() => parsePorts(ranges(17))).toThrow();
});

test('an empty list is valid for binding ports', () => {
	expect(parsePorts('')).toEqual([]);
});

test('udp flow idle is packed in seconds and must be whole', () => {
	const { config } = udpConfigValue({ host: 'h', ports: '53', flowIdle: '30' });
	expect(anyUnpack(config!, UdpConfigSchema)).toMatchObject({ flowIdle: { seconds: 30n } });
	expect(() => udpConfigValue({ host: 'h', ports: '53', flowIdle: '1.5' })).toThrow();
});

test('spaces inside a range are rejected', () => {
	expect(() => parsePorts('22 - 80')).toThrow();
});
