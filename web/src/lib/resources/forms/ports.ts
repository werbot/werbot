import { create } from '@bufbuild/protobuf';
import { PortRangeSchema, type PortRange } from '#lib/gen/werbot/connector/v1/ports_pb.js';

// Mirrors connector.ValidatePorts: at most 16 ranges, ports 1-65535, no overlap.
const maxRanges = 16;
const maxPort = 65535;

// parsePorts reads "22, 8000-8100"; an empty text is no ranges. Throws on anything the server would refuse.
export function parsePorts(text: string): PortRange[] {
	const parts = text.split(',').map((s) => s.trim()).filter(Boolean);
	if (parts.length > maxRanges) throw new Error('too many ranges');
	const out: PortRange[] = [];
	for (const part of parts) {
		const m = /^(\d+)(?:-(\d+))?$/.exec(part);
		if (!m) throw new Error(`invalid range ${part}`);
		const port = Number(m[1]);
		const end = m[2] === undefined ? port : Number(m[2]);
		if (port < 1 || end < port || end > maxPort) throw new Error(`invalid range ${part}`);
		if (out.some((o) => port <= (o.endPort || o.port) && o.port <= end)) throw new Error(`range ${part} overlaps`);
		out.push(create(PortRangeSchema, { port, endPort: end === port ? 0 : end }));
	}
	return out;
}

// formatPorts is the inverse of parsePorts.
export const formatPorts = (ranges: PortRange[]) => ranges.map((r) => (r.endPort ? `${r.port}-${r.endPort}` : `${r.port}`)).join(', ');
