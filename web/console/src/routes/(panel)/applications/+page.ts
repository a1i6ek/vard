import { applicationsApi } from '#lib/api/index.js';
import { LIST_PAGE_SIZE } from '#lib/query/index.js';
import { fromApi, requireAnywhere } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, parent, url }) => {
	requireAnywhere((await parent()).admin, 'applications.read');

	const search = url.searchParams.get('search')?.trim() ?? '';
	const profile = url.searchParams.get('profile') ?? '';

	const page = await fromApi(
		applicationsApi.list({ search, profile, limit: LIST_PAGE_SIZE }, fetch)
	);

	return { page, search, profile };
};
