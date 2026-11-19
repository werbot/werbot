import { page } from '$app/state';

/** Whether a login is the signed-in user's: their avatar wears the logo gold (`.wb-me` in theme.css). */
export const isMe = (login?: string) => !!login && login === page.data.session?.user?.login;
