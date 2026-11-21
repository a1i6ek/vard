import type {
	PublicKeyCredentialCreationOptionsJSON,
	PublicKeyCredentialRequestOptionsJSON
} from '@simplewebauthn/browser';
import { request, type Fetch } from './client';
import type { components } from './schema';

/** The account API's types, each its schema in api/openapi/account/schemas.yaml, generated
 *  into schema.d.ts by `make generate`. */
type Schemas = components['schemas'];

export type Application = Schemas['Application'];

export type Organization = Schemas['Organization'];

export type LoginStep = Schemas['LoginStep'];

/**
 * What these pages may offer, from the application's login flow or the default. The server
 * enforces it regardless.
 */
export type LoginOptions = Schemas['LoginOptions'];

export type PublicLanguage = Schemas['PublicLanguage'];

export type SocialProvider = Schemas['SocialProvider'];

export type SSOConnection = Schemas['SSOConnection'];

export type SignInRequest = Schemas['SignInRequest'];

export type CodeChallenge = Schemas['CodeChallenge'];

export type SignedIn = Schemas['SignedIn'];

export type SecondFactorMethod = Schemas['SecondFactorMethod'];

/** What the second-factor page offers. The sign-in itself is named by a cookie the page never
    sees. */
export type SecondFactorChallenge = Schemas['SecondFactorChallenge'];

export type SecondFactorAnswer = Schemas['SecondFactorAnswer'];

export type Factor = Schemas['Factor'];

export type TwoStep = Schemas['TwoStep'];

/** Recovery codes, the one time they are shown; empty when none were made. */
export type RecoveryCodes = Schemas['RecoveryCodes'];

/** An authenticator being set up: the key, and the address its QR code draws. */
export type AuthenticatorSecret = Schemas['AuthenticatorSecret'];

export type User = Schemas['User'];

export type Session = Schemas['Session'];

export type ConnectedApplication = Schemas['ConnectedApplication'];

/** One thing an application asks for. A standard scope is described by the page; an API's
    scope comes with the API's own words. */
export type RequestedScope = Schemas['RequestedScope'];

export type ConsentPage = Schemas['ConsentPage'];

/** A device waiting to be allowed; `agreements` when allowing it also accepts the
    organization's terms, which its application's flow asks for. */
export type DevicePage = Schemas['DevicePage'];

/** The Agreements or Consent page's answer, for the sign-in it names. */
export type StepAnswer = Schemas['StepAnswer'];

export type RegisterInput = Schemas['RegisterInput'];

const encode = encodeURIComponent;

export const signIn = {
	request: (handle: string, fetch?: Fetch) =>
		request<SignInRequest>(`/account/requests/${encode(handle)}`, { fetch }),

	application: (clientId: string, fetch?: Fetch) =>
		request<{ application: Application }>(`/account/applications/${encode(clientId)}`, { fetch }),

	organization: (fetch?: Fetch) =>
		request<{ organization: Organization }>('/account/organization', { fetch }),

	socialProviders: (fetch?: Fetch) =>
		request<{ providers: SocialProvider[] }>('/account/social-providers', { fetch }),

	ssoConnections: (fetch?: Fetch) =>
		request<{ connections: SSOConnection[]; available: boolean }>('/account/sso', { fetch }),

	discoverSSO: (email: string) =>
		request<{ connection: SSOConnection; enforced: boolean }>('/account/sso/discover', {
			method: 'POST',
			body: { email }
		}),

	languages: (fetch?: Fetch) =>
		request<{ languages: PublicLanguage[]; default: string }>('/account/languages', { fetch }),

	languageText: (code: string, fetch?: Fetch) =>
		request<{ language: PublicLanguage; messages: Record<string, string> }>(
			`/account/languages/${encodeURIComponent(code)}`,
			{ fetch }
		),

	loginOptions: (handle: string, fetch?: Fetch) =>
		request<{ login: LoginOptions }>(
			handle ? `/account/login-options?request=${encode(handle)}` : '/account/login-options',
			{ fetch }
		),

	login: (body: { request: string; email: string; password: string; remember: boolean }) =>
		request<SignedIn>('/account/login', { method: 'POST', body }),

	register: (body: RegisterInput) =>
		request<SignedIn>('/account/register', { method: 'POST', body }),

	code: (body: { handle: string; code: string }) =>
		request<SignedIn>('/account/login/code', { method: 'POST', body }),

	resendCode: (body: { handle: string }) =>
		request<{ code: CodeChallenge }>('/account/login/code/resend', { method: 'POST', body }),

	forgotPassword: (body: { request: string; email: string; language: string }) =>
		request<{ status: string }>('/account/forgot-password', { method: 'POST', body }),

	checkReset: (token: string, fetch?: Fetch) =>
		request<{ valid: boolean }>(`/account/reset-password?token=${encode(token)}`, { fetch }),

	resetPassword: (body: { token: string; password: string }) =>
		request<{ status: string }>('/account/reset-password', { method: 'POST', body }),

	verifyEmail: (body: { token: string; password?: string }) =>
		request<{ status: string }>('/account/verify-email', { method: 'POST', body }),

	consent: (handle: string, fetch?: Fetch) =>
		request<{ consent: ConsentPage }>(`/account/consent?request=${encode(handle)}`, { fetch }),

	answerConsent: (body: StepAnswer) =>
		request<{ redirect_to: string }>('/account/consent', { method: 'POST', body }),

	answerAgreements: (body: StepAnswer) =>
		request<{ redirect_to: string }>('/account/agreements', { method: 'POST', body }),

	beginPasskey: (body: { request: string; remember: boolean }) =>
		request<{ handle: string; passkey: PublicKeyCredentialRequestOptionsJSON }>(
			'/account/login/passkey',
			{ method: 'POST', body }
		),

	finishPasskey: (body: { handle: string; response: unknown }) =>
		request<SignedIn>('/account/login/passkey/finish', { method: 'POST', body }),

	secondFactor: (fetch?: Fetch) =>
		request<{ second_factor: SecondFactorChallenge }>('/account/login/second-factor', { fetch }),

	submitSecondFactor: (body: SecondFactorAnswer) =>
		request<SignedIn>('/account/login/second-factor', { method: 'POST', body }),

	/** Setting up a first factor while signing in, for an application that requires one; the
	    waiting sign-in's cookie names whose. */
	beginEnrolAuthenticator: () =>
		request<{ authenticator: AuthenticatorSecret }>('/account/login/second-factor/authenticator', {
			method: 'POST'
		}),

	confirmEnrolAuthenticator: (code: string) =>
		request<SignedIn>('/account/login/second-factor/authenticator/confirm', {
			method: 'POST',
			body: { code }
		}),

	beginEnrolPasskey: () =>
		request<{ passkey: PublicKeyCredentialCreationOptionsJSON }>(
			'/account/login/second-factor/passkeys',
			{ method: 'POST' }
		),

	finishEnrolPasskey: (body: { label: string; response: unknown }) =>
		request<SignedIn>('/account/login/second-factor/passkeys/finish', { method: 'POST', body }),

	/** Gives up the waiting sign-in ("Not now"); `redirect_to` is the application, told the
	    person did not continue, or the account page. */
	leaveSecondFactor: () =>
		request<SignedIn>('/account/login/second-factor/leave', { method: 'POST' }),

	/** Signs this browser out. The confirmation page passes on what the
	    application's logout named; `location` is where the server says to go. */
	logout: (body?: {
		client_id: string;
		post_logout_redirect_uri: string;
		state: string;
		ui_locales: string;
	}) => request<{ status: string; location: string }>('/account/logout', { method: 'POST', body })
};

export const account = {
	me: (fetch?: Fetch) => request<{ user: User }>('/account/me', { fetch }),

	updateProfile: (body: { first_name: string; last_name: string }) =>
		request<{ user: User }>('/account/me', { method: 'PATCH', body }),

	changePassword: (body: { current_password: string; new_password: string }) =>
		request<{ status: string }>('/account/password', { method: 'POST', body }),

	changeEmail: (body: { email: string; current_password: string }) =>
		request<{ status: string }>('/account/email', { method: 'POST', body }),

	sessions: (fetch?: Fetch) => request<{ sessions: Session[] }>('/account/sessions', { fetch }),

	device: (userCode: string, fetch?: Fetch) =>
		request<{ device: DevicePage }>(`/account/device?user_code=${encode(userCode)}`, { fetch }),

	answerDevice: (body: { user_code: string; yes: boolean; accept_terms: boolean }) =>
		request<void>('/account/device', { method: 'POST', body }),

	/** Starts a step-up of this session for the application a device asks for; `second_factor`
	    when the second-factor page is to finish it. */
	stepUp: (userCode: string) =>
		request<SignedIn>('/account/step-up', { method: 'POST', body: { user_code: userCode } }),

	twoStep: (fetch?: Fetch) => request<{ two_step: TwoStep }>('/account/two-step', { fetch }),

	beginAuthenticator: (currentPassword: string) =>
		request<{ authenticator: AuthenticatorSecret }>('/account/two-step/authenticator', {
			method: 'POST',
			body: { current_password: currentPassword }
		}),

	confirmAuthenticator: (code: string) =>
		request<RecoveryCodes>('/account/two-step/authenticator/confirm', {
			method: 'POST',
			body: { code }
		}),

	beginPasskey: (currentPassword: string) =>
		request<{ passkey: PublicKeyCredentialCreationOptionsJSON }>('/account/two-step/passkeys', {
			method: 'POST',
			body: { current_password: currentPassword }
		}),

	finishPasskey: (body: { label: string; response: unknown }) =>
		request<RecoveryCodes>('/account/two-step/passkeys/finish', { method: 'POST', body }),

	removeFactor: (id: string, currentPassword: string) =>
		request<void>(`/account/two-step/factors/${encodeURIComponent(id)}`, {
			method: 'DELETE',
			body: { current_password: currentPassword }
		}),

	newRecoveryCodes: (currentPassword: string) =>
		request<RecoveryCodes>('/account/two-step/recovery-codes', {
			method: 'POST',
			body: { current_password: currentPassword }
		}),

	endSession: (id: string) =>
		request<void>(`/account/sessions/${encode(id)}`, { method: 'DELETE' }),

	applications: (fetch?: Fetch) =>
		request<{ applications: ConnectedApplication[] }>('/account/connected-applications', {
			fetch
		}),

	disconnect: (clientId: string) =>
		request<void>(`/account/connected-applications/${encode(clientId)}`, { method: 'DELETE' })
};
