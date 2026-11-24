import { Code, ConnectError } from '@connectrpc/connect';
import { toast } from '#lib/ui/index.js';
import { m } from '#lib/paraglide/messages.js';

// One error-feedback rule for the whole app:
// - every server error (an action, a load, a public auth form) → failToast(e): a toast, no alert banner;
//   a modal or drawer stays open after a failed action so the user can retry;
// - a failed load also swaps its content area for <LoadFailed onretry={…} />, a neutral empty state
//   with a Retry button, so the page does not go blank; a successful reload brings the content back;
// - client-side validation of one field → that field's error text (FormField `error`);
// - informational banners (recording state, resource health, DB-IP credit, hints) are not errors.

const messages: Partial<Record<Code, () => string>> = {
	[Code.InvalidArgument]: m.error_invalid_argument,
	[Code.Unauthenticated]: m.error_unauthenticated,
	[Code.PermissionDenied]: m.error_permission_denied,
	[Code.NotFound]: m.error_not_found,
	[Code.AlreadyExists]: m.error_already_exists,
	[Code.FailedPrecondition]: m.error_failed_precondition,
	[Code.ResourceExhausted]: m.error_resource_exhausted,
	[Code.Unavailable]: m.error_unavailable
};

export function errorMessage(e: unknown): string {
	return (messages[ConnectError.from(e).code] ?? m.error_unknown)();
}

/** Toasts a server error: an error object goes through errorMessage, a string shows as is. */
export function failToast(e: unknown): void {
	toast({ tone: 'alert', text: typeof e === 'string' ? e : errorMessage(e) });
}

/** An InvalidArgument error keeps its full server text (for example a glob or a Cedar position); any other error passes unchanged to failToast. */
export function serverText(e: unknown): unknown {
	const c = ConnectError.from(e);
	return c.code === Code.InvalidArgument && c.rawMessage ? c.rawMessage : e;
}
