import { Code, ConnectError } from '@connectrpc/connect';
import { m } from '#lib/paraglide/messages.js';

const messages: Partial<Record<Code, () => string>> = {
	[Code.InvalidArgument]: m.error_invalid_argument,
	[Code.Unauthenticated]: m.error_unauthenticated,
	[Code.PermissionDenied]: m.error_permission_denied,
	[Code.NotFound]: m.error_not_found,
	[Code.AlreadyExists]: m.error_already_exists,
	[Code.FailedPrecondition]: m.error_failed_precondition,
	[Code.Unavailable]: m.error_unavailable
};

export function errorMessage(e: unknown): string {
	return (messages[ConnectError.from(e).code] ?? m.error_unknown)();
}
