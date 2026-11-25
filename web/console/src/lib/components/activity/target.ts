import { resolve } from '$app/paths';
import type { ActivityTarget } from '#lib/api/index.js';

/**
 * Where the panel shows the record an entry is about: its own page, or the
 * list it is in, searched for it. Only for a target whose name the reader
 * was sent, which the server leaves out for one they may not see; nothing
 * for a kind with no page of its own.
 */
export function targetHref(target: ActivityTarget | null | undefined): string | null {
	if (!target?.name) return null;

	const search = (base: string) => `${base}?search=${encodeURIComponent(target.name ?? '')}`;

	switch (target.type) {
		case 'api':
			return resolve('/(panel)/apis/[id]', { id: target.id });
		case 'login_flow':
			return resolve('/(panel)/flows/[id]', { id: target.id });
		case 'user':
			return search(resolve('/(panel)/users'));
		case 'application':
			return search(resolve('/(panel)/applications'));
		case 'admin_user':
			return search(resolve('/(panel)/admins'));
		default:
			return null;
	}
}
