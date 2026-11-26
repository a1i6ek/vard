import { nextUnlessSignedIn } from '#lib/session.js';
import type { PageLoad } from './$types';

// Someone already signed in has an account; where a new one would have gone
// is where they are sent.
export const load: PageLoad = async ({ url, fetch }) => ({
	next: await nextUnlessSignedIn(fetch, url)
});
