import { timestampDate, type Timestamp } from '@bufbuild/protobuf/wkt';
import type { GetRunResponse } from '#lib/gen/werbot/v1/agent_pb.js';
import { reasonName } from '#lib/policies/reasons.js';

export type TimelineKind = 'tool_call' | 'session' | 'approval' | 'denial';
export type TimelineFilter = 'all' | 'denials' | 'approvals' | 'dangerous';

export interface TimelineItem {
	kind: TimelineKind;
	at: Date;
	id: string;
	title: string;
	status: string;
	classes: string[];
	sessionId?: string;
}

const order: TimelineKind[] = ['tool_call', 'session', 'approval', 'denial'];
const dangerous = new Set(['destructive', 'privilege', 'secrets', 'egress']);
const noNames = new Map<string, string>();
const when = (t?: Timestamp) => (t ? timestampDate(t) : new Date(0));

/** The four lists of a run as one list by time; equal times keep the kind order. */
export function buildTimeline(r: GetRunResponse): TimelineItem[] {
	const items: TimelineItem[] = [
		...r.toolCalls.map((c): TimelineItem => ({ kind: 'tool_call', at: when(c.startedAt), id: c.id, title: c.tool, status: c.status, classes: [] })),
		...r.sessions.map(
			(s): TimelineItem => ({ kind: 'session', at: when(s.startedAt), id: s.id, title: s.command || s.resourceName, status: s.endReason, classes: s.classes, sessionId: s.id })
		),
		...r.approvals.map((a): TimelineItem => ({ kind: 'approval', at: when(a.createdAt), id: a.id, title: a.command, status: a.status, classes: a.classes })),
		// a denial has no id of its own: its position in the list names it
		...r.denials.map((d, i): TimelineItem => ({ kind: 'denial', at: when(d.at), id: `denial:${i}`, title: d.command, status: d.reasons.map((x) => reasonName(x, noNames)).join(', '), classes: d.classes }))
	];
	return items.sort((a, b) => a.at.getTime() - b.at.getTime() || order.indexOf(a.kind) - order.indexOf(b.kind));
}

const keep: Record<TimelineFilter, (i: TimelineItem) => boolean> = {
	all: () => true,
	denials: (i) => i.kind === 'denial' || (i.kind === 'tool_call' && i.status === 'denied'),
	approvals: (i) => i.kind === 'approval',
	dangerous: (i) => i.classes.some((c) => dangerous.has(c))
};

export const filterTimeline = (items: TimelineItem[], f: TimelineFilter): TimelineItem[] => items.filter(keep[f]);
