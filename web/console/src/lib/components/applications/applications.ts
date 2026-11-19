import type { IconName } from '#lib/components/ui/index.js';
import type {
	ApplicationInput,
	ApplicationProfile,
	AuthMethod,
	GrantType,
	Scope
} from '#lib/api/index.js';

/** The five kinds of client (ADR 0005), and what each is. */
export const profiles: Record<
	ApplicationProfile,
	{ label: string; description: string; icon: IconName; public: boolean }
> = {
	web: {
		label: 'Web application',
		description: 'A server-side app (a BFF) that keeps a key or a secret',
		icon: 'ri-computer-line',
		public: false
	},
	spa: {
		label: 'Single-page app',
		description: 'Runs in the browser with no backend; no secret, PKCE instead',
		icon: 'ri-apps-line',
		public: true
	},
	native: {
		label: 'Mobile app',
		description: 'iOS or Android, sent back to https or its own scheme; PKCE',
		icon: 'ri-smartphone-line',
		public: true
	},
	desktop: {
		label: 'Desktop or command line',
		description: 'Sent back to 127.0.0.1 on any port, or signs in with a code; PKCE',
		icon: 'ri-terminal-box-line',
		public: true
	},
	machine: {
		label: 'Machine',
		description: 'A service or an agent calling APIs as itself, signing with its own key',
		icon: 'ri-server-line',
		public: false
	}
};

export const authMethods: { value: AuthMethod; label: string; description: string }[] = [
	{
		value: 'private_key_jwt',
		label: 'private_key_jwt',
		description: 'A JWT signed with a key only the application holds (recommended)'
	},
	{
		value: 'client_secret_basic',
		label: 'client_secret_basic',
		description: 'The secret in an HTTP Basic header'
	},
	{
		value: 'client_secret_post',
		label: 'client_secret_post',
		description: 'The secret in the token request body'
	}
];

export const grants: { value: GrantType; description: string }[] = [
	{ value: 'authorization_code', description: 'Sign users in with a redirect and a code' },
	{ value: 'refresh_token', description: 'Get new tokens without asking the user again' },
	{ value: 'client_credentials', description: 'Act as the application itself, with no user' },
	{
		value: 'urn:ietf:params:oauth:grant-type:device_code',
		description: 'Sign users in on a TV or a command line: they enter a short code on their phone'
	}
];

/** The grants that get tokens for a user. */
export const userGrants: GrantType[] = [
	'authorization_code',
	'urn:ietf:params:oauth:grant-type:device_code'
];

export const scopes: { value: Scope; description: string }[] = [
	{ value: 'openid', description: 'Sign in with OpenID Connect and get an ID token' },
	{ value: 'profile', description: "The user's name" },
	{ value: 'email', description: "The user's address and whether it is verified" },
	{ value: 'offline_access', description: 'A refresh token, to stay signed in' },
	{ value: 'roles', description: "The user's roles in this application" }
];

/** The settings a new application of a type starts with: the choices RFC
    9700 recommends, so the form is right before anything is changed. */
export function blank(profile: ApplicationProfile): ApplicationInput {
	const machine = profile === 'machine';
	return {
		name: '',
		description: '',
		profile,
		logo_url: '',
		website_url: '',
		privacy_url: '',
		terms_url: '',
		token_auth_method: profiles[profile].public
			? 'none'
			: machine
				? 'private_key_jwt'
				: 'client_secret_basic',
		grant_types: machine ? ['client_credentials'] : ['authorization_code', 'refresh_token'],
		redirect_uris: [],
		post_logout_redirect_uris: [],
		allow_development_redirects: false,
		backchannel_logout_uri: '',
		jwks: null,
		jwks_uri: '',
		scopes: machine ? [] : ['openid', 'profile', 'email'],
		require_pushed_authorization_requests: false,
		access_token_lifetime: 60 * 60,
		id_token_lifetime: 60 * 60,
		refresh_token_lifetime: 30 * 24 * 60 * 60,
		refresh_token_idle_lifetime: 14 * 24 * 60 * 60,
		assert_roles: true,
		require_role_assignment: false,
		is_enabled: true,
		allow_registration: !machine,
		login_flow_id: ''
	};
}

export function lines(text: string): string[] {
	return text
		.split('\n')
		.map((line) => line.trim())
		.filter((line) => line !== '');
}

export const ADMIN_CLI = 'admin-cli';
