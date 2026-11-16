import { expect, test } from 'vitest';
import { configFormFor, sshConfigValue } from './index';
import SshConfigForm from './SshConfigForm.svelte';
import JsonConfigForm from './JsonConfigForm.svelte';

test('ssh has a dedicated form, unknown types fall back to JSON', () => {
	expect(configFormFor('ssh')).toBe(SshConfigForm);
	expect(configFormFor('echo')).toBe(JsonConfigForm);
});

test('ssh value is packed as Any of werbot.connector.ssh.v1.Config', () => {
	expect(sshConfigValue({ host: 'h', port: 22, login: 'root' }).config?.typeUrl).toBe(
		'type.googleapis.com/werbot.connector.ssh.v1.Config'
	);
});
