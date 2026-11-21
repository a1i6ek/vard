import type { SSOConnection } from '#lib/api/index.js';

/** A certificate this close to expiring is worth a warning. */
export const EXPIRY_WARNING = 30 * 24 * 3600 * 1000;

/** When the provider's signing certificate expires, if that is within the warning or past;
 *  null otherwise. */
export function certificateExpiring(
	connection: SSOConnection | null,
	now: Date = new Date()
): Date | null {
	const expires = connection?.identity_provider?.certificate_expires;
	if (!expires) return null;

	const when = new Date(expires);
	return when.getTime() - now.getTime() < EXPIRY_WARNING ? when : null;
}
