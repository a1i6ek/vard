import type { JsonWebKey as Jwk, JsonWebKeySet } from '#lib/api/index.js';

/** Where a private_key_jwt client's public keys come from, read from what was pasted. */
export type KeySource =
	| { kind: 'none' }
	| { kind: 'jwks'; jwks: JsonWebKeySet }
	| { kind: 'jwks_uri'; uri: string }
	| { kind: 'problem'; problem: string };

/** The algorithms the server verifies a client assertion with. */
const ACCEPTED = ['ES256', 'PS256', 'RS256'];

const PRIVATE_KEY =
	'That is a private key. Paste only the public half: the private key stays with the application.';

/**
 * Reads a public key in any form an administrator is likely to have: a PEM public key, a JWK, a
 * JWK Set, or the https address the application publishes its keys at. A private key is refused,
 * so it is never sent.
 */
export async function readPublicKey(text: string): Promise<KeySource> {
	const trimmed = text.trim();
	if (trimmed === '') return { kind: 'none' };

	if (/^https?:\/\//i.test(trimmed)) return { kind: 'jwks_uri', uri: trimmed };

	if (/-----BEGIN [A-Z ]*PRIVATE KEY-----/.test(trimmed)) return problem(PRIVATE_KEY);
	if (trimmed.startsWith('-----BEGIN PUBLIC KEY-----')) return fromPem(trimmed);

	let parsed: unknown;
	try {
		parsed = JSON.parse(trimmed);
	} catch {
		return problem(
			'Paste a PEM public key (-----BEGIN PUBLIC KEY-----), a JWK, a JWK Set, or an https address.'
		);
	}
	if (!parsed || typeof parsed !== 'object') return problem('This is not a key.');

	const keys = Array.isArray((parsed as JsonWebKeySet).keys)
		? (parsed as JsonWebKeySet).keys
		: [parsed as Jwk];
	return fromKeys(keys);
}

/** The same, for what needs no decoding: a saved key set, or an address. */
export function readSavedKeys(jwks: JsonWebKeySet | null, uri: string): KeySource {
	if (uri) return { kind: 'jwks_uri', uri };
	if (jwks && jwks.keys.length > 0) return { kind: 'jwks', jwks };
	return { kind: 'none' };
}

/** The text a saved key source is shown as in the box. */
export function keyText(source: KeySource): string {
	if (source.kind === 'jwks_uri') return source.uri;
	if (source.kind === 'jwks') return JSON.stringify(source.jwks, null, 2);
	return '';
}

/** One line on each key: what it is and its id. */
export function describeKeys(jwks: JsonWebKeySet): string {
	return jwks.keys
		.map((key) => `${key.alg ?? key.kty}${key.kid ? ` · ${shortKid(key.kid)}` : ''}`)
		.join(', ');
}

function shortKid(kid: string): string {
	return kid.length > 12 ? `${kid.slice(0, 12)}…` : kid;
}

async function fromKeys(keys: Jwk[]): Promise<KeySource> {
	if (keys.length === 0) return problem('The key set has no keys.');

	const out: Jwk[] = [];
	for (const key of keys) {
		if (!key || typeof key !== 'object' || typeof key.kty !== 'string') {
			return problem('Each key needs a kty: EC or RSA.');
		}
		if ('d' in key) return problem(PRIVATE_KEY);

		const alg = key.alg ?? defaultAlg(key);
		if (!alg || !ACCEPTED.includes(alg)) {
			return problem('Use an ES256 (EC P-256), PS256 or RS256 (RSA) key.');
		}
		out.push({ ...key, alg, use: 'sig', kid: key.kid || (await thumbprint(key)) });
	}
	return { kind: 'jwks', jwks: { keys: out } };
}

async function fromPem(pem: string): Promise<KeySource> {
	const body = pem.replace(/-----[A-Z ]+-----/g, '').replace(/\s+/g, '');
	let der: Uint8Array<ArrayBuffer>;
	try {
		der = Uint8Array.from(atob(body), (char) => char.charCodeAt(0));
	} catch {
		return problem('The PEM key is damaged: its base64 does not decode.');
	}

	const tries: [EcKeyImportParams | RsaHashedImportParams, string][] = [
		[{ name: 'ECDSA', namedCurve: 'P-256' }, 'ES256'],
		[{ name: 'RSASSA-PKCS1-v1_5', hash: 'SHA-256' }, 'RS256']
	];
	for (const [params, alg] of tries) {
		try {
			const imported = await crypto.subtle.importKey('spki', der, params, true, ['verify']);
			const { kty, crv, x, y, n, e } = await crypto.subtle.exportKey('jwk', imported);
			return fromKeys([{ kty: kty!, crv, x, y, n, e, alg } as Jwk]);
		} catch {
			// Not this kind of key: the next.
		}
	}
	return problem('Use an EC P-256 or RSA public key.');
}

function defaultAlg(key: Jwk): string | undefined {
	if (key.kty === 'EC') return key.crv === 'P-256' ? 'ES256' : undefined;
	if (key.kty === 'RSA') return 'RS256';
	return undefined;
}

/** The key's RFC 7638 thumbprint, as its id when it came with none. */
async function thumbprint(key: Jwk): Promise<string> {
	const members =
		key.kty === 'EC'
			? { crv: key.crv, kty: key.kty, x: key.x, y: key.y }
			: { e: key.e, kty: key.kty, n: key.n };
	const digest = await crypto.subtle.digest(
		'SHA-256',
		new TextEncoder().encode(JSON.stringify(members))
	);
	return base64url(new Uint8Array(digest));
}

function base64url(bytes: Uint8Array): string {
	return btoa(String.fromCharCode(...bytes))
		.replace(/\+/g, '-')
		.replace(/\//g, '_')
		.replace(/=+$/, '');
}

function problem(text: string): KeySource {
	return { kind: 'problem', problem: text };
}

/** A key the application signs with, made here: the private half for the application, which
    leaves only as a file to save, and the public half to register. */
export type GeneratedKey = {
	kid: string;
	publicKey: Jwk;
	privateKey: JsonWebKey & { kid: string };
};

export async function generateKey(): Promise<GeneratedKey> {
	const pair = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
		'sign',
		'verify'
	]);
	const { kty, crv, x, y } = await crypto.subtle.exportKey('jwk', pair.publicKey);
	const publicKey = { kty: kty!, crv, x, y, alg: 'ES256', use: 'sig' } as Jwk;
	const kid = await thumbprint(publicKey);
	const privateKey = await crypto.subtle.exportKey('jwk', pair.privateKey);
	delete privateKey.key_ops;
	delete privateKey.ext;
	return {
		kid,
		publicKey: { ...publicKey, kid },
		privateKey: { ...privateKey, kid, alg: 'ES256' }
	};
}

/** The file the application keeps: its private key, and what it sends it to. A new application
    has no client ID or issuer yet; the Connect tab gives both once it is made. */
export function keyFile(key: GeneratedKey, clientId: string, issuer: string) {
	const base = issuer.replace(/\/+$/, '');
	return {
		...(clientId ? { client_id: clientId } : {}),
		key_id: key.kid,
		private_key: key.privateKey,
		...(base ? { issuer: base, token_endpoint: `${base}/oauth2/token` } : {})
	};
}
