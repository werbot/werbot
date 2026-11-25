import { Code, ConnectError } from '@connectrpc/connect';
import { expect, test, vi } from 'vitest';
import { finishStepUp, requestStepUp, stepUp, stepUpInterceptor } from './stepup.svelte.js';

const req = (typeName = 'werbot.v1.MemberService', stream = false) => ({ stream, service: { typeName } }) as never;
const denied = (msg: string) => new ConnectError(msg, Code.PermissionDenied);

test('retries exactly once after a confirmed step-up', async () => {
	const next = vi.fn().mockRejectedValueOnce(denied('mfa required')).mockResolvedValueOnce('ok');
	const ask = vi.fn().mockResolvedValue(true);
	await expect(stepUpInterceptor(ask)(next)(req())).resolves.toBe('ok');
	expect(ask).toHaveBeenCalledOnce();
	expect(next).toHaveBeenCalledTimes(2);
});

test('a second mfa required after the retry shows no dialog and passes through', async () => {
	const next = vi.fn().mockRejectedValue(denied('mfa required'));
	const ask = vi.fn().mockResolvedValue(true);
	await expect(stepUpInterceptor(ask)(next)(req())).rejects.toThrow('mfa required');
	expect(ask).toHaveBeenCalledOnce();
	expect(next).toHaveBeenCalledTimes(2);
});

test('another PermissionDenied is not a step-up', async () => {
	const next = vi.fn().mockRejectedValue(denied('forbidden'));
	const ask = vi.fn();
	await expect(stepUpInterceptor(ask)(next)(req())).rejects.toThrow('forbidden');
	expect(ask).not.toHaveBeenCalled();
	expect(next).toHaveBeenCalledOnce();
});

test('a cancelled dialog rethrows the original error without retry', async () => {
	const next = vi.fn().mockRejectedValue(denied('mfa required'));
	await expect(stepUpInterceptor(async () => false)(next)(req())).rejects.toThrow('mfa required');
	expect(next).toHaveBeenCalledOnce();
});

test('AuthService calls never open the dialog', async () => {
	const next = vi.fn().mockRejectedValue(denied('mfa required'));
	const ask = vi.fn();
	await expect(stepUpInterceptor(ask)(next)(req('werbot.v1.AuthService'))).rejects.toThrow();
	expect(ask).not.toHaveBeenCalled();
});

test('concurrent requests share one dialog', async () => {
	const a = requestStepUp();
	const b = requestStepUp();
	expect(stepUp.open).toBe(true);
	finishStepUp(true);
	await expect(Promise.all([a, b])).resolves.toEqual([true, true]);
	expect(stepUp.open).toBe(false);
});
