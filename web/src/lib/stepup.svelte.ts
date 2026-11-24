import { Code, ConnectError, type Interceptor } from '@connectrpc/connect';
import { AuthService } from '#lib/gen/werbot/v1/auth_pb.js';

/** The exact PermissionDenied text of the fresh-MFA gate. */
const MFA_REQUIRED = 'mfa required';

/** StepUpDialog (mounted once in the app layout) shows while `open` is true. */
export const stepUp = $state({ open: false });

let waiting: Promise<boolean> | null = null;
let settle: (ok: boolean) => void = () => {};

/** Opens the step-up dialog; concurrent callers share one dialog. Resolves true once a factor was confirmed. */
export function requestStepUp(): Promise<boolean> {
	waiting ??= new Promise<boolean>((resolve) => {
		settle = resolve;
		stepUp.open = true;
	});
	return waiting;
}

export function finishStepUp(ok: boolean): void {
	stepUp.open = false;
	waiting = null;
	settle(ok);
}

/**
 * "mfa required" → the step-up dialog → the request again, once: the retry is not
 * wrapped, so a second refusal reaches the caller. AuthService calls (the step-up
 * itself) never open the dialog.
 */
export function stepUpInterceptor(ask: () => Promise<boolean> = requestStepUp): Interceptor {
	return (next) => async (req) => {
		try {
			return await next(req);
		} catch (e) {
			const err = ConnectError.from(e);
			const gate = !req.stream && req.service.typeName !== AuthService.typeName && err.code === Code.PermissionDenied && err.rawMessage === MFA_REQUIRED;
			if (!gate || !(await ask())) throw e;
			return next(req);
		}
	};
}
