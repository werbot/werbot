import { describe, expect, it } from 'vitest';
import { modeOf } from './effects';

describe('modeOf', () => {
	it('allows an empty list', () => expect(modeOf([])).toEqual({ mode: 'allow', mixed: false }));
	it('takes the one effect of the list', () => expect(modeOf(['deny', 'deny'])).toEqual({ mode: 'deny', mixed: false }));
	it('flags a list with both effects', () => expect(modeOf(['deny', 'allow'])).toEqual({ mode: 'deny', mixed: true }));
});
