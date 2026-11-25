import { create } from '@bufbuild/protobuf';
import { timestampFromDate } from '@bufbuild/protobuf/wkt';
import { describe, expect, it } from 'vitest';
import { GetRunResponseSchema, type GetRunResponse } from '#lib/gen/werbot/v1/agent_pb.js';
import { buildTimeline, filterTimeline } from './timeline.js';

const at = (s: number) => timestampFromDate(new Date(Date.UTC(2026, 10, 23, 10, 0, s)));

const run = (): GetRunResponse =>
	create(GetRunResponseSchema, {
		toolCalls: [{ id: 't1', tool: 'exec', status: 'denied', startedAt: at(1) }],
		sessions: [{ id: 's1', command: 'ls', classes: ['read'], startedAt: at(2) }],
		denials: [{ command: 'cat /etc/shadow', classes: ['secrets'], reasons: ['agent:baseline:secrets'], at: at(0) }],
		approvals: [{ id: 'a1', command: 'sudo ls', classes: ['write', 'privilege'], status: 'pending', createdAt: at(3) }]
	});

describe('timeline', () => {
	it('merges and sorts four lists by time', () => {
		expect(buildTimeline(run()).map((i) => [i.kind, i.id])).toEqual([
			['denial', 'denial:0'],
			['tool_call', 't1'],
			['session', 's1'],
			['approval', 'a1']
		]);
	});

	it('orders equal times by kind', () => {
		const r = create(GetRunResponseSchema, {
			denials: [{ command: 'x', at: at(5) }],
			sessions: [{ id: 's', startedAt: at(5) }],
			toolCalls: [{ id: 't', tool: 'exec', startedAt: at(5) }]
		});
		expect(buildTimeline(r).map((i) => i.kind)).toEqual(['tool_call', 'session', 'denial']);
	});

	it('denials filter keeps denials and denied tool calls', () => {
		const r = run();
		r.toolCalls.push(...create(GetRunResponseSchema, { toolCalls: [{ id: 't2', tool: 'exec', status: 'ok', startedAt: at(4) }] }).toolCalls);
		const kept = filterTimeline(buildTimeline(r), 'denials').map((i) => i.id);
		expect(kept).toEqual(['denial:0', 't1']);
	});

	it('dangerous filter uses classes', () => {
		const kept = filterTimeline(buildTimeline(run()), 'dangerous').map((i) => i.kind);
		expect(kept).toEqual(['denial', 'approval']);
	});

	it('approvals filter keeps approvals; all keeps everything', () => {
		const items = buildTimeline(run());
		expect(filterTimeline(items, 'approvals').map((i) => i.id)).toEqual(['a1']);
		expect(filterTimeline(items, 'all')).toHaveLength(4);
	});
});
