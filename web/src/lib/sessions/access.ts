import { Role } from '#lib/gen/werbot/v1/user_pb.js';

/** Watch and terminate: owner and admin for any session, anyone else only for their own. */
export function canControlSession(role: Role | undefined, viewerLogin: string | undefined, sessionLogin: string): boolean {
	if (role === Role.OWNER || role === Role.ADMIN) return true;
	return !!viewerLogin && viewerLogin === sessionLogin;
}
