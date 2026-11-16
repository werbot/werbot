import type { Component } from 'svelte';
import { create } from '@bufbuild/protobuf';
import { anyPack, type Any } from '@bufbuild/protobuf/wkt';
import { ConfigSchema } from '#lib/gen/werbot/connector/ssh/v1/ssh_pb.js';
import type { Resource, ResourceType } from '#lib/gen/werbot/v1/resource_pb.js';
import SshConfigForm from './SshConfigForm.svelte';
import JsonConfigForm from './JsonConfigForm.svelte';

export type ConfigValue = { config?: Any; configJson?: string };
export type ConfigFormProps = { type: ResourceType; initial?: Resource; onchange: (v: ConfigValue) => void };

export const configForms: Record<string, Component<ConfigFormProps>> = { ssh: SshConfigForm };

export function configFormFor(type: string): Component<ConfigFormProps> {
	return configForms[type] ?? JsonConfigForm;
}

export function sshConfigValue(v: { host: string; port: number; login: string }): ConfigValue {
	return { config: anyPack(ConfigSchema, create(ConfigSchema, v)) };
}
