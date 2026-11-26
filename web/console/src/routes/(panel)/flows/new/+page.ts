import { flowsApi } from '#lib/api/index.js';
import { fromApi, requirePermission } from '#lib/api/load.js';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ fetch, parent }) => {
	requirePermission((await parent()).admin, 'login_flows.write');

	const { flows } = await fromApi(flowsApi.list(fetch));

	return { taken: flows.map((one) => one.slug) };
};
