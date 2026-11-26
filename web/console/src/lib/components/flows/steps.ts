import type { LoginStep } from '#lib/api/index.js';

/** Everything a flow is, as the panel edits it and the API takes it. */
export type FlowDraft = {
	name: string;
	slug: string;
	description: string;
	is_default: boolean;
	is_enabled: boolean;
	steps: LoginStep[];
	allow_sign_in: boolean;
	allow_registration: boolean;
	allow_password_reset: boolean;
	allow_remember_me: boolean;
	verify_email_on_register: boolean;
	require_verified_email: boolean;
	require_second_factor: boolean;
	allow_email_change: boolean;
	session_lifetime_hours: number;
};

export function draftOf(flow: FlowDraft): FlowDraft {
	return {
		name: flow.name,
		slug: flow.slug,
		description: flow.description,
		is_default: flow.is_default,
		is_enabled: flow.is_enabled,
		steps: [...flow.steps],
		allow_sign_in: flow.allow_sign_in,
		allow_registration: flow.allow_registration,
		allow_password_reset: flow.allow_password_reset,
		allow_remember_me: flow.allow_remember_me,
		verify_email_on_register: flow.verify_email_on_register,
		require_verified_email: flow.require_verified_email,
		require_second_factor: flow.require_second_factor,
		allow_email_change: flow.allow_email_change,
		session_lifetime_hours: flow.session_lifetime_hours
	};
}

/** A new flow: an email and a password, with the Social page's providers
    beside them, and a verified address before the account is used. */
export const NEW_FLOW: FlowDraft = {
	name: '',
	slug: '',
	description: '',
	is_default: false,
	is_enabled: true,
	steps: ['identifier', 'password', 'social'],
	allow_sign_in: true,
	allow_registration: true,
	allow_password_reset: true,
	allow_remember_me: true,
	verify_email_on_register: false,
	require_verified_email: true,
	require_second_factor: false,
	allow_email_change: false,
	session_lifetime_hours: 24 * 14
};

/** The order steps are kept in. The server asks only whether a flow has a
    step, never where, so a switch keeps them in this one order. */
const ORDER: LoginStep[] = [
	'identifier',
	'password',
	'email_code',
	'social',
	'totp',
	'terms',
	'consent'
];

/** The flow's steps with one turned on or off. The address step is always
    there: a flow that does not ask who is signing in has nobody to sign in. */
export function withStep(steps: LoginStep[], step: LoginStep, on: boolean): LoginStep[] {
	const kept = new Set(steps);
	if (on) kept.add(step);
	else kept.delete(step);
	kept.add('identifier');

	return ORDER.filter((one) => kept.has(one));
}

const PROOF_STEPS: LoginStep[] = ['password', 'social'];

export const MAX_SESSION_HOURS = 24 * 90;

const SLUG = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;

/** What stops the flow being saved, in the words the page shows; the server
    holds it to the same rules. */
export function problemsOf(draft: FlowDraft): string[] {
	const problems: string[] = [];

	if (draft.name.trim() === '') problems.push('Name the flow.');
	if (!SLUG.test(draft.slug))
		problems.push('The identifier has to be lower case letters, numbers and dashes.');
	if (!draft.steps.some((step) => PROOF_STEPS.includes(step)))
		problems.push('Turn on passwords or other accounts: without one, nobody can sign in.');
	if (draft.require_second_factor && !draft.steps.includes('totp'))
		problems.push('Turn on two-step verification to require it.');
	if (draft.is_default && !draft.is_enabled)
		problems.push('The default flow cannot be turned off.');
	if (
		!Number.isInteger(draft.session_lifetime_hours) ||
		draft.session_lifetime_hours < 1 ||
		draft.session_lifetime_hours > MAX_SESSION_HOURS
	)
		problems.push('A session has to last between one hour and ninety days.');

	return problems;
}

export function slugFrom(name: string): string {
	return name
		.toLowerCase()
		.normalize('NFKD')
		.replace(/[^a-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, 64);
}

export function freeSlug(wanted: string, taken: string[]): string {
	const base = wanted || 'flow';
	if (!taken.includes(base)) return base;

	for (let n = 2; ; n++) {
		const candidate = `${base.slice(0, 60)}-${n}`;
		if (!taken.includes(candidate)) return candidate;
	}
}

/** How people sign in under a flow, in a few words for the list. */
export function methodsOf(steps: LoginStep[]): string[] {
	const methods: string[] = [];
	if (steps.includes('password')) methods.push('Password');
	if (steps.includes('email_code')) methods.push('Emailed code');
	if (steps.includes('social')) methods.push('Other accounts');
	if (steps.includes('totp')) methods.push('Two-step');
	return methods;
}
