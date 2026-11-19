import { Code, ConnectError } from '@connectrpc/connect';
import { afterEach, expect, test } from 'vitest';
import { dismissToast, toasts } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';
import { failToast } from './errors.js';

afterEach(() => [...toasts].forEach((t) => dismissToast(t.id)));

test('an error object toasts its mapped message', () => {
	failToast(new ConnectError('owner', Code.FailedPrecondition));
	expect(toasts.map((t) => [t.tone, t.text])).toEqual([['alert', m.error_failed_precondition()]]);
});

test('a string toasts as is', () => {
	failToast('Copy failed');
	expect(toasts.map((t) => [t.tone, t.text])).toEqual([['alert', 'Copy failed']]);
});
