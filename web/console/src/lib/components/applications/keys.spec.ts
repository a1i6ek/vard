import { describe, expect, it } from 'vitest';
import { generateKey, keyFile, readPublicKey } from './keys';

async function pemOf(key: CryptoKey): Promise<string> {
	const der = new Uint8Array(await crypto.subtle.exportKey('spki', key));
	const body = btoa(String.fromCharCode(...der)).replace(/(.{64})/g, '$1\n');
	return `-----BEGIN PUBLIC KEY-----\n${body}\n-----END PUBLIC KEY-----`;
}

const ec = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, true, [
	'sign',
	'verify'
]);
const ecPublic = await crypto.subtle.exportKey('jwk', ec.publicKey);
const ecPrivate = await crypto.subtle.exportKey('jwk', ec.privateKey);
const rsa = await crypto.subtle.generateKey(
	{
		name: 'RSASSA-PKCS1-v1_5',
		modulusLength: 2048,
		publicExponent: new Uint8Array([1, 0, 1]),
		hash: 'SHA-256'
	},
	true,
	['sign', 'verify']
);
const p384 = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-384' }, true, [
	'sign',
	'verify'
]);

describe('readPublicKey', () => {
	const cases: {
		name: string;
		text: () => Promise<string> | string;
		want: string;
		alg?: string;
	}[] = [
		{ name: 'nothing is no key', text: () => '  ', want: 'none' },
		{
			name: 'an https address is where the keys are published',
			text: () => 'https://jobs.example.com/.well-known/jwks.json',
			want: 'jwks_uri'
		},
		{
			name: 'a JWK on its own becomes a set',
			text: () => JSON.stringify(ecPublic),
			want: 'jwks',
			alg: 'ES256'
		},
		{
			name: 'a JWK Set is taken as it is',
			text: () => JSON.stringify({ keys: [ecPublic] }),
			want: 'jwks',
			alg: 'ES256'
		},
		{
			name: 'an EC PEM public key',
			text: () => pemOf(ec.publicKey),
			want: 'jwks',
			alg: 'ES256'
		},
		{
			name: 'an RSA PEM public key',
			text: () => pemOf(rsa.publicKey),
			want: 'jwks',
			alg: 'RS256'
		},
		{
			name: 'a private JWK is refused, never sent',
			text: () => JSON.stringify(ecPrivate),
			want: 'problem'
		},
		{
			name: 'a private PEM key is refused, never sent',
			text: () => '-----BEGIN PRIVATE KEY-----\nMIGHAgEA\n-----END PRIVATE KEY-----',
			want: 'problem'
		},
		{
			name: 'a P-384 key is not one the server verifies with',
			text: async () => JSON.stringify(await crypto.subtle.exportKey('jwk', p384.publicKey)),
			want: 'problem'
		},
		{ name: 'an empty set', text: () => '{"keys": []}', want: 'problem' },
		{ name: 'words', text: () => 'my key', want: 'problem' }
	];

	for (const tt of cases) {
		it(tt.name, async () => {
			const got = await readPublicKey(await tt.text());
			expect(got.kind).toBe(tt.want);
			if (got.kind === 'jwks') {
				expect(got.jwks.keys[0].alg).toBe(tt.alg);
				expect(got.jwks.keys[0].kid).toBeTruthy();
				expect(got.jwks.keys[0]).not.toHaveProperty('d');
			}
		});
	}
});

describe('generateKey', () => {
	it('registers only the public half, and the file signs what it verifies', async () => {
		const key = await generateKey();
		expect(key.publicKey).not.toHaveProperty('d');
		expect(key.privateKey.d).toBeTruthy();
		expect(key.publicKey.kid).toBe(key.kid);

		const registered = await readPublicKey(JSON.stringify({ keys: [key.publicKey] }));
		expect(registered.kind).toBe('jwks');

		const file = keyFile(key, 'nightly-job', 'https://id.example.com/');
		expect(file.token_endpoint).toBe('https://id.example.com/oauth2/token');

		const signer = await crypto.subtle.importKey(
			'jwk',
			file.private_key,
			{ name: 'ECDSA', namedCurve: 'P-256' },
			false,
			['sign']
		);
		const verifier = await crypto.subtle.importKey(
			'jwk',
			key.publicKey as JsonWebKey,
			{ name: 'ECDSA', namedCurve: 'P-256' },
			false,
			['verify']
		);
		const data = new TextEncoder().encode('assertion');
		const signature = await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, signer, data);
		expect(
			await crypto.subtle.verify({ name: 'ECDSA', hash: 'SHA-256' }, verifier, signature, data)
		).toBe(true);
	});
});
