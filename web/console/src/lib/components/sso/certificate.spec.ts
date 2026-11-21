import { describe, expect, it } from 'vitest';
import type { SSOConnection } from '#lib/api/index.js';
import { certificateExpiring } from './certificate';

const now = new Date('2026-11-21T00:00:00Z');
const expiring = (expires?: string) =>
	({ identity_provider: expires ? { certificate_expires: expires } : undefined }) as SSOConnection;

describe("a provider's certificate", () => {
	it('is not mentioned while it has more than thirty days', () =>
		expect(certificateExpiring(expiring('2027-01-15T00:00:00Z'), now)).toBeNull());
	it('is warned of within thirty days', () =>
		expect(certificateExpiring(expiring('2026-12-01T00:00:00Z'), now)).toEqual(
			new Date('2026-12-01T00:00:00Z')
		));
	it('is warned of once expired', () =>
		expect(certificateExpiring(expiring('2026-11-01T00:00:00Z'), now)).not.toBeNull());
	it('is not mentioned when the metadata names no expiry', () =>
		expect(certificateExpiring(expiring(), now)).toBeNull());
});
