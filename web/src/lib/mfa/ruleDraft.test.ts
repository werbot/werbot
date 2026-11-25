import { expect, test } from 'vitest';
import { fromDraft, maxAgeInvalid, toDraft } from './ruleDraft.js';

test('a new rule asks a fresh factor within 60 s, any strength', () => {
	const d = toDraft();
	expect(d).toMatchObject({ maxAge: '60s', strength: 'any', by: 'resources' });
	expect(fromDraft(d)).toMatchObject({ strength: 'any', maxAge: { seconds: 60n } });
});

test('an existing rule round-trips max age and the labels selector', () => {
	const d = toDraft({ name: 'prod', strength: 'webauthn', maxAge: { seconds: 3600n, nanos: 0 }, resourceSelector: { labels: { env: 'prod' }, resourceIds: [] } } as never);
	expect(d).toMatchObject({ name: 'prod', maxAge: '1h', strength: 'webauthn', by: 'labels' });
	expect(fromDraft(d).resourceSelector).toEqual({ resourceIds: [], labels: { env: 'prod' } });
});

test('max age must be 30s to 24h', () => {
	expect(maxAgeInvalid('60s')).toBe(false);
	expect(maxAgeInvalid('30s')).toBe(false);
	expect(maxAgeInvalid('24h')).toBe(false);
	expect(maxAgeInvalid('29s')).toBe(true);
	expect(maxAgeInvalid('25h')).toBe(true);
	expect(maxAgeInvalid('abc')).toBe(true);
});
