/** What a machine client is given: everything to get a token and call an API with it. */
export type MachineSetup = {
	issuer: string;
	clientId: string;
	/** The API's identifier, sent as resource. */
	resource: string;
	/** Where the API answers, when the panel knows: the admin API's address. */
	apiUrl?: string;
};

export function tokenEndpoint(issuer: string): string {
	return `${issuer.replace(/\/+$/, '')}/oauth2/token`;
}

/**
 * A complete client in one file, for Node 20+ or Bun with no dependencies: it reads the key file
 * the Credentials section saved, gets a DPoP-bound token with client_credentials and
 * private_key_jwt, answering the server's DPoP nonce, and calls the API with it.
 */
export function exampleScript(setup: MachineSetup): string {
	const issuer = setup.issuer.replace(/\/+$/, '');
	const call = setup.apiUrl
		? `
// Call the API: a fresh proof for every request, bound to the token.
const url = API_URL + '/users'; // needs users.read
const answer = await withNonce(async (nonce) =>
	fetch(url, {
		headers: {
			authorization: \`DPoP \${tokens.access_token}\`,
			dpop: await proof('GET', url, nonce, tokens.access_token)
		}
	})
);
console.log(answer.status, await answer.json());
`
		: `
// Call the API with: authorization: DPoP <access_token>, and a fresh
// proof(method, url, nonce, access_token) as the dpop header.
console.log(tokens.token_type, tokens.scope, tokens.expires_in);
`;

	return `// node client.mjs key.json
import { readFile } from 'node:fs/promises';

const ISSUER = '${issuer}';
const TOKEN_ENDPOINT = '${tokenEndpoint(issuer)}';
const CLIENT_ID = '${setup.clientId}';
const RESOURCE = '${setup.resource}';${setup.apiUrl ? `\nconst API_URL = '${setup.apiUrl}';` : ''}

const key = JSON.parse(await readFile(process.argv[2], 'utf8'));
const encoder = new TextEncoder();
const b64url = (bytes) => Buffer.from(bytes).toString('base64url');
const json64 = (value) => b64url(encoder.encode(JSON.stringify(value)));
const now = () => Math.floor(Date.now() / 1000);

async function jwt(header, payload, privateKey) {
	const input = \`\${json64(header)}.\${json64(payload)}\`;
	const signature = await crypto.subtle.sign(
		{ name: 'ECDSA', hash: 'SHA-256' }, privateKey, encoder.encode(input)
	);
	return \`\${input}.\${b64url(new Uint8Array(signature))}\`;
}

// The application's key, which proves it is CLIENT_ID (private_key_jwt).
const clientKey = await crypto.subtle.importKey(
	'jwk', key.private_key, { name: 'ECDSA', namedCurve: 'P-256' }, false, ['sign']
);

// A DPoP key for this run: the token is bound to it, and useless without it.
const dpop = await crypto.subtle.generateKey(
	{ name: 'ECDSA', namedCurve: 'P-256' }, false, ['sign', 'verify']
);
const { kty, crv, x, y } = await crypto.subtle.exportKey('jwk', dpop.publicKey);

const assertion = () =>
	jwt(
		{ alg: 'ES256', typ: 'JWT', kid: key.key_id },
		{ iss: CLIENT_ID, sub: CLIENT_ID, aud: ISSUER, jti: crypto.randomUUID(), iat: now(), exp: now() + 60 },
		clientKey
	);

async function proof(method, url, nonce, accessToken) {
	const ath = accessToken
		? b64url(new Uint8Array(await crypto.subtle.digest('SHA-256', encoder.encode(accessToken))))
		: undefined;
	return jwt(
		{ alg: 'ES256', typ: 'dpop+jwt', jwk: { kty, crv, x, y } },
		{ jti: crypto.randomUUID(), htm: method, htu: url, iat: now(), nonce, ath },
		dpop.privateKey
	);
}

// The server answers a proof without its nonce with one to send instead.
async function withNonce(send) {
	const first = await send(undefined);
	const nonce = first.headers.get('dpop-nonce');
	return (first.status === 400 || first.status === 401) && nonce ? send(nonce) : first;
}

const tokenAnswer = await withNonce(async (nonce) =>
	fetch(TOKEN_ENDPOINT, {
		method: 'POST',
		headers: {
			'content-type': 'application/x-www-form-urlencoded',
			dpop: await proof('POST', TOKEN_ENDPOINT, nonce)
		},
		body: new URLSearchParams({
			grant_type: 'client_credentials',
			resource: RESOURCE,
			client_assertion_type: 'urn:ietf:params:oauth:client-assertion-type:jwt-bearer',
			client_assertion: await assertion()
		})
	})
);
const tokens = await tokenAnswer.json();
if (!tokenAnswer.ok) throw new Error(JSON.stringify(tokens));
${call}`;
}
