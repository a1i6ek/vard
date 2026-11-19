import type {
	Application,
	ApplicationInput,
	ApplicationProfile,
	GrantType,
	JsonWebKeySet,
	Scope
} from '#lib/api/index.js';
import { blank, lines, profiles } from './applications';

const MINUTE = 60;
const DAY = 24 * 60 * MINUTE;

/** The application dialog's fields. Lists and lifetimes are edited as text
    (one URI a line, minutes and days) and turned back into the API's shape
    by `toInput`. */
export class ApplicationForm {
	input = $state<ApplicationInput>(blank('web'));
	redirects = $state('');
	logoutRedirects = $state('');
	accessMinutes = $state('60');
	idMinutes = $state('60');
	refreshDays = $state('30');
	/** A private_key_jwt client's keys: a JWK Set as JSON, or an address. */
	keysFrom = $state<'jwks' | 'jwks_uri'>('jwks');
	keySet = $state('');

	kind = $derived(profiles[this.input.profile]);
	/** A machine client has no user: nothing to redirect, no ID token, and it
	    signs with its own key. */
	machine = $derived(this.input.profile === 'machine');
	usesKeys = $derived(this.input.token_auth_method === 'private_key_jwt');
	/** Why the pasted key set cannot be sent, or '' when it can. */
	keySetProblem = $derived(
		this.usesKeys && this.keysFrom === 'jwks' ? keySetProblem(this.keySet) : ''
	);
	ready = $derived(this.input.name.trim() !== '' && this.keySetProblem === '');

	fill(from: Application | ApplicationInput | null): void {
		const input: ApplicationInput = from
			? { ...from, login_flow_id: from.login_flow_id ?? '' }
			: blank('web');

		this.input = {
			...input,
			grant_types: [...input.grant_types],
			scopes: [...input.scopes],
			redirect_uris: [...input.redirect_uris],
			post_logout_redirect_uris: [...input.post_logout_redirect_uris]
		};
		this.redirects = input.redirect_uris.join('\n');
		this.logoutRedirects = input.post_logout_redirect_uris.join('\n');
		this.accessMinutes = String(Math.round(input.access_token_lifetime / MINUTE));
		this.idMinutes = String(Math.round(input.id_token_lifetime / MINUTE));
		this.refreshDays = String(Math.round(input.refresh_token_lifetime / DAY));
		this.keysFrom = input.jwks_uri ? 'jwks_uri' : 'jwks';
		this.keySet = input.jwks ? JSON.stringify(input.jwks, null, 2) : '';
	}

	/** Starts again from the new profile's defaults, keeping how it is presented. */
	changeProfile(profile: ApplicationProfile): void {
		const { name, description, logo_url, website_url, privacy_url, terms_url } = this.input;
		this.fill({
			...blank(profile),
			name,
			description,
			logo_url,
			website_url,
			privacy_url,
			terms_url
		});
	}

	setGrant(grant: GrantType, on: boolean): void {
		this.input.grant_types = toggled(this.input.grant_types, grant, on);
	}

	setScope(scope: Scope, on: boolean): void {
		this.input.scopes = toggled(this.input.scopes, scope, on);
	}

	toInput(): ApplicationInput {
		return {
			...this.input,
			name: this.input.name.trim(),
			description: this.input.description.trim(),
			backchannel_logout_uri: this.input.backchannel_logout_uri.trim(),
			redirect_uris: lines(this.redirects),
			post_logout_redirect_uris: lines(this.logoutRedirects),
			access_token_lifetime: Number(this.accessMinutes) * MINUTE,
			id_token_lifetime: Number(this.idMinutes) * MINUTE,
			refresh_token_lifetime: Number(this.refreshDays) * DAY,
			jwks: this.usesKeys && this.keysFrom === 'jwks' ? parseKeySet(this.keySet) : null,
			jwks_uri: this.usesKeys && this.keysFrom === 'jwks_uri' ? this.input.jwks_uri.trim() : ''
		};
	}
}

/** A JWK Set as pasted, or null when it is not one. */
function parseKeySet(text: string): JsonWebKeySet | null {
	try {
		const parsed: unknown = JSON.parse(text);
		if (parsed && typeof parsed === 'object' && Array.isArray((parsed as JsonWebKeySet).keys)) {
			return parsed as JsonWebKeySet;
		}
	} catch {
		// Not JSON: said by keySetProblem.
	}
	return null;
}

function keySetProblem(text: string): string {
	if (text.trim() === '') return 'Paste the public keys as a JWK Set: {"keys": [ … ]}.';
	return parseKeySet(text) ? '' : 'This is not a JWK Set: it needs {"keys": [ … ]}.';
}

function toggled<T extends string>(list: T[], value: T, on: boolean): T[] {
	if (on) return list.includes(value) ? list : [...list, value];
	return list.filter((item) => item !== value);
}
