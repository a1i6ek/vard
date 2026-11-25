import { isRedirect } from '@sveltejs/kit';
import { account } from '#lib/api/index.js';
import { asUser } from '#lib/session.js';
import type { PageLoad } from './$types';

/** A summary that could not be read is left out, not the page with it; a
    session that has ended still goes to sign in. */
async function orNull<T>(read: Promise<T>): Promise<T | null> {
	try {
		return await read;
	} catch (err) {
		if (isRedirect(err)) throw err;
		return null;
	}
}

export const load: PageLoad = async ({ fetch, url }) => {
	const [twoStep, sessions, applications] = await Promise.all([
		orNull(asUser(() => account.twoStep(fetch), url).then((body) => body.two_step)),
		orNull(asUser(() => account.sessions(fetch), url).then((body) => body.sessions)),
		orNull(asUser(() => account.applications(fetch), url).then((body) => body.applications))
	]);

	return { twoStep, sessions, applications };
};
