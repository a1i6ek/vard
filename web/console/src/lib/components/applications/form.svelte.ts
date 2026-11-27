import type {
	Application,
	ApplicationInput,
	ApplicationProfile,
	GrantType,
	Scope
} from '#lib/api/index.js';
import { androidApps, androidLines, blank, lines, profiles } from './applications';
import { keyText, readSavedKeys, type KeySource } from './keys';

const MINUTE = 60;
const DAY = 24 * 60 * MINUTE;

/** The application dialog's fields. Lists and lifetimes are edited as text
    (one URI a line, minutes and days) and turned back into the API's shape
    by `toInput`. */
export class ApplicationForm {
	input = $state<ApplicationInput>(blank('web'));
	redirects = $state('');
	logoutRedirects = $state('');
	iosAppIds = $state('');
	android = $state('');
	accessMinutes = $state('60');
	idMinutes = $state('60');
	refreshDays = $state('30');
	refreshIdleDays = $state('14');
	/** A private_key_jwt client's public keys as typed or pasted: a PEM key, a
	    JWK or JWK Set, or the address they are published at. */
	keyText = $state('');
	/** What keyText was read as (keys.ts reads it, PEM keys asynchronously). */
	keys = $state<KeySource>({ kind: 'none' });

	kind = $derived(profiles[this.input.profile]);
	/** A machine client has no user: nothing to redirect, no ID token, and it
	    signs with its own key. */
	machine = $derived(this.input.profile === 'machine');
	usesKeys = $derived(this.input.token_auth_method === 'private_key_jwt');
	/** Why the pasted key cannot be sent, or '' when it can. No key at all is
	    allowed: the application cannot get tokens until it has one. */
	keyProblem = $derived(this.usesKeys && this.keys.kind === 'problem' ? this.keys.problem : '');
	ready = $derived(this.input.name.trim() !== '' && this.keyProblem === '');

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
		this.iosAppIds = (input.ios_app_ids ?? []).join('\n');
		this.android = androidLines(input.android_apps ?? []);
		this.accessMinutes = String(Math.round(input.access_token_lifetime / MINUTE));
		this.idMinutes = String(Math.round(input.id_token_lifetime / MINUTE));
		this.refreshDays = String(Math.round(input.refresh_token_lifetime / DAY));
		this.refreshIdleDays = String(Math.round(input.refresh_token_idle_lifetime / DAY));
		// Worked out once, not read back from this.keys: fill runs in the
		// dialog's effect, which must not come to depend on the keys it sets.
		const keys = readSavedKeys(input.jwks ?? null, input.jwks_uri ?? '');
		this.keys = keys;
		this.keyText = keyText(keys);
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
			ios_app_ids: lines(this.iosAppIds),
			android_apps: androidApps(this.android),
			access_token_lifetime: Number(this.accessMinutes) * MINUTE,
			id_token_lifetime: Number(this.idMinutes) * MINUTE,
			refresh_token_lifetime: Number(this.refreshDays) * DAY,
			refresh_token_idle_lifetime: Number(this.refreshIdleDays) * DAY,
			jwks: this.usesKeys && this.keys.kind === 'jwks' ? this.keys.jwks : null,
			jwks_uri: this.usesKeys && this.keys.kind === 'jwks_uri' ? this.keys.uri : ''
		};
	}
}

function toggled<T extends string>(list: T[], value: T, on: boolean): T[] {
	if (on) return list.includes(value) ? list : [...list, value];
	return list.filter((item) => item !== value);
}
