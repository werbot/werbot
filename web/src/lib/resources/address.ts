import type { PortRange } from '#lib/gen/werbot/connector/v1/ports_pb.js';
import { formatPorts } from './forms/ports.js';

/** "host:port" (or "host:22, 8000-8100") from a resource's config JSON; empty when it has no host. */
export function address(configJson: string): string {
	let cfg: { host?: string; port?: number; ports?: PortRange[] };
	try {
		cfg = JSON.parse(configJson);
	} catch {
		return '';
	}
	if (!cfg?.host) return '';
	const ports = cfg.port ? String(cfg.port) : formatPorts(cfg.ports ?? []);
	return ports ? `${cfg.host}:${ports}` : cfg.host;
}
