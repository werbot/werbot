import type { Member } from '#lib/gen/werbot/v1/member_pb.js';
import type { SelectOption } from '#lib/ui/index.js';

/** userOptions lists the members that have a user as "Name (login)" select options. */
export const userOptions = (members: Member[]): SelectOption[] =>
	members.flatMap((x) => (x.user ? [{ value: x.user.id, label: `${x.user.name} (${x.user.login})` }] : []));
