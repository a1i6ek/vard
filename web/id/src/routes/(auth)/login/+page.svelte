<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';

	import { onMount, tick } from 'svelte';

	import {
		ApiError,
		requiredSSO,
		signIn,
		type CodeChallenge,
		type SSOConnection,
		type SignedIn
	} from '#lib/api/index.js';

	import {
		AuthCard,
		CodeForm,
		PasskeySignIn,
		SocialButtons,
		SSOButtons
	} from '#lib/components/auth/index.js';
	import {
		Alert,
		Button,
		Checkbox,
		Form,
		Note,
		PageTitle,
		PasswordField,
		TextField
	} from '#lib/components/ui/index.js';
	import { LIMITS } from '#lib/constants.js';
	import { messageOf, useTranslator } from '#lib/i18n/index.js';
	import { authHref, leaveTo, secondFactorHref, ssoHref } from '#lib/utils/links.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const t = useTranslator();

	const app = $derived(data.application);

	const flow = $derived(data.login);
	const offerSocial = $derived(flow.steps.includes('social'));
	const offerPassword = $derived(flow.steps.includes('password'));
	/** Passkeys are added where two-step verification is offered, so that is where they sign in. */
	const offerPasskey = $derived(flow.steps.includes('totp'));
	const offerRegistration = $derived(
		offerPassword && flow.allow_registration && app?.allow_registration === true
	);
	const closed = $derived(!flow.allow_sign_in);
	const offerRemember = $derived(flow.allow_remember_me);

	// The application may already know who is signing in (login_hint).
	// svelte-ignore state_referenced_locally
	let email = $state(data.signInRequest?.login_hint ?? '');
	let password = $state('');
	/** "Stay signed in": off by default, because the machine somebody is
	    signing in on is not known to be theirs. */
	let remember = $state(false);
	let error = $state('');
	let notice = $state('');
	let submitting = $state(false);
	let redirecting = $state('');
	let challenge = $state<CodeChallenge | null>(null);

	/** Identifier first: the address, then how that address signs in. Every
	    address that is not at a single sign-on domain gets the same second
	    step, so the page tells nothing about who has an account. */
	let stage = $state<'identify' | 'password'>('identify');
	/** A connection that owns the address's domain without requiring itself,
	    offered beside the password. */
	let offeredSSO = $state<SSOConnection | null>(null);

	const canIdentify = $derived(email.trim() !== '' && !submitting);
	const canSubmit = $derived(email.trim() !== '' && password !== '' && !submitting);

	/** Looks the domain up: a connection that requires itself takes the
	    address there; one that does not is offered; otherwise, the password. */
	async function identify() {
		if (!canIdentify) return;

		error = '';
		notice = '';
		submitting = true;
		offeredSSO = null;

		try {
			const { connection, enforced } = await signIn.discoverSSO(email.trim());
			if (enforced) {
				redirecting = connection.name;
				leaveTo(
					ssoHref(connection.slug, { request: data.request, next: data.next, email: email.trim() })
				);
				return;
			}
			offeredSSO = connection;
		} catch (err) {
			if (!(err instanceof ApiError && err.code === 'no_sso_connection')) {
				error = messageOf(err, t);
				submitting = false;
				return;
			}
		}

		submitting = false;
		stage = 'password';
		await tick();
		document.getElementById('login-password')?.querySelector('input')?.focus();
	}

	function changeAddress() {
		stage = 'identify';
		password = '';
		error = '';
		offeredSSO = null;
	}

	// An application that already knows who is signing in (login_hint)
	// skips the first step.
	onMount(() => {
		if (data.signInRequest?.login_hint && offerPassword) void identify();
	});

	async function arrive(result: SignedIn, next: string) {
		if (result.password_change_required && result.reset_token) {
			let query = `token=${encodeURIComponent(result.reset_token)}&reason=temporary`;
			if (data.request) query += `&request=${encodeURIComponent(data.request)}`;
			// replace so the reset token does not stay in the browser's
			// history: this entry replaces the sign-in page rather than stacking
			// on it, and Back does not return to a URL carrying a live token.
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved, with a query
			await goto(`${resolve('reset-password')}?${query}`, { replace: true });
			return;
		}

		if (result.second_factor) {
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved, with a query
			await goto(secondFactorHref(data.request, next), { replace: true });
			return;
		}

		if (result.code) {
			challenge = result.code;
			password = '';
			submitting = false;
			return;
		}

		if (result.redirect_to) {
			leaveTo(result.redirect_to);
			return;
		}

		// Signed in to the account itself. One navigation, which reloads the
		// data on the way: a separate refreshAll would re-run this page's
		// load, whose redirect for a signed-in user races this goto. `next` is
		// a checked path on this site, but one with no route makes goto reject,
		// so it is loaded the plain way and shows the not-found page.
		try {
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- a checked path on this site
			await goto(next, { refreshAll: true, replace: true });
		} catch {
			window.location.replace(next);
		}
	}

	function signedInWithCode(result: SignedIn) {
		void arrive(result, data.next);
	}

	function codeExpired(message: string) {
		challenge = null;
		submitting = false;
		password = '';
		error = message;
	}

	async function submit() {
		if (!canSubmit) return;

		error = '';
		notice = '';
		submitting = true;

		// Read before anything is awaited: once the session changes, this
		// page's data can be replaced under it.
		const next = data.next;

		try {
			const result = await signIn.login({
				request: data.request ?? '',
				email: email.trim(),
				password,
				remember
			});

			await arrive(result, next);
		} catch (err) {
			const sso = requiredSSO(err);
			if (sso) {
				redirecting = sso.name;
				password = '';
				leaveTo(ssoHref(sso.slug, { request: data.request, next, email: email.trim() }));
				return;
			}

			if (err instanceof ApiError && err.code === 'email_not_verified') {
				notice = messageOf(err, t);
			} else {
				error = messageOf(err, t);
			}
			password = '';
			submitting = false;
		}
	}
</script>

<PageTitle parts={[t('login.head'), app?.name]} />

{#if data.expired}
	<AuthCard
		organization={data.organization}
		title={t('login.expired_title')}
		subtitle={t('login.expired_subtitle')}
	>
		<Alert tone="info">{t('login.expired_body')}</Alert>

		{#snippet below()}
			{t('login.expired_or')}
			<a href={resolve('login')}>{t('login.expired_link')}</a>
		{/snippet}
	</AuthCard>
{:else if closed}
	<AuthCard
		organization={data.organization}
		application={app}
		title={t('login.closed_title')}
		subtitle={app ? t('login.subtitle_app', { app: app.name }) : t('login.subtitle_account')}
	>
		<Alert tone="warning">{t('login.closed_body')}</Alert>
	</AuthCard>
{:else if challenge}
	<AuthCard
		organization={data.organization}
		application={app}
		title={t('login.code_title')}
		subtitle={t('login.code_subtitle', { email: challenge.email })}
	>
		<CodeForm {challenge} onsignedin={signedInWithCode} onexpired={codeExpired} />

		{#snippet below()}
			<a href={resolve('login')}>{t('login.code_back')}</a>
		{/snippet}
	</AuthCard>
{:else}
	<AuthCard
		organization={data.organization}
		application={app}
		title={t('login.title')}
		subtitle={app ? t('login.subtitle_app', { app: app.name }) : t('login.subtitle_account')}
	>
		{#if offerPassword && stage === 'identify'}
			<Form onsubmit={identify}>
				{#if error}<Alert>{error}</Alert>{/if}
				{#if notice}<Alert tone="info">{notice}</Alert>{/if}
				{#if redirecting}
					<Alert tone="info">{t('login.sso_redirecting', { name: redirecting })}</Alert>
				{/if}

				<TextField
					label={t('field.email')}
					bind:value={email}
					type="email"
					name="email"
					autocomplete={offerPasskey ? 'username webauthn' : 'username'}
					autocapitalize="none"
					spellcheck={false}
					maxlength={LIMITS.email}
					disabled={submitting}
				/>

				<Button type="submit" block loading={submitting} disabled={!canIdentify}>
					{t('login.next')}
				</Button>
			</Form>
		{:else if offerPassword}
			<Form onsubmit={submit}>
				{#if error}<Alert>{error}</Alert>{/if}
				{#if notice}<Alert tone="info">{notice}</Alert>{/if}

				<div class="identity">
					<i class="ri-user-line" aria-hidden="true"></i>
					<span class="address">{email.trim()}</span>
					<Button variant="link" onclick={changeAddress} disabled={submitting}>
						{t('login.change')}
					</Button>
				</div>
				<!-- The address, for a password manager to save beside the
				     password: the field it was typed in has gone. -->
				<input
					class="username"
					type="email"
					name="email"
					autocomplete="username"
					value={email.trim()}
					readonly
					tabindex="-1"
					aria-hidden="true"
				/>

				{#if offeredSSO}
					<Button
						block
						variant="subtle"
						onclick={() =>
							leaveTo(
								ssoHref(offeredSSO!.slug, {
									request: data.request,
									next: data.next,
									email: email.trim()
								})
							)}
					>
						{t('login.continue_with', { name: offeredSSO.name })}
					</Button>
					<Note align="center" size="sm">{t('login.or_password')}</Note>
				{/if}

				<div id="login-password">
					<PasswordField
						label={t('field.password')}
						bind:value={password}
						disabled={submitting}
						aside={flow.allow_password_reset
							? { label: t('login.forgot'), href: authHref('/forgot-password', data.request) }
							: undefined}
					/>
				</div>

				{#if offerRemember}
					<Checkbox bind:checked={remember} disabled={submitting}>
						{t('login.remember')}
						<span class="aside">{t('login.remember_hint')}</span>
					</Checkbox>
				{/if}

				<Button type="submit" block loading={submitting} disabled={!canSubmit}>
					{submitting ? t('login.submitting') : t('login.submit')}
				</Button>
			</Form>
		{/if}

		{#if !offerPassword && error}<Alert>{error}</Alert>{/if}

		{#if offerPasskey && stage === 'identify'}
			<PasskeySignIn
				request={data.request}
				{remember}
				disabled={submitting}
				onsignedin={(result) => arrive(result, data.next)}
				onerror={(message) => (error = message)}
				autofill={offerPassword}
			/>
		{/if}

		<SocialButtons
			providers={offerSocial && stage === 'identify' ? data.socialProviders : []}
			request={data.request}
			next={data.next}
			label={t('login.social')}
			disabled={submitting}
		/>

		<SSOButtons
			connections={stage === 'identify' ? data.ssoConnections : []}
			request={data.request}
			next={data.next}
			disabled={submitting}
		/>

		{#if data.ssoAvailable && stage === 'identify'}
			<Note align="center" size="sm">
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a href={authHref('/sso', data.request)}>{t('login.sso')}</a>
			</Note>
		{/if}

		{#snippet below()}
			{#if offerRegistration && data.request}
				{t('login.no_account')}
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
				<a href={authHref('/register', data.request)}>{t('login.create_one')}</a>
			{/if}
		{/snippet}
	</AuthCard>
{/if}

<style>
	.aside {
		display: block;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.identity {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-height: var(--field-height);
		padding: 0 var(--nudge-12);
		border-radius: var(--radius-field);
		background: var(--color-field);
	}

	.address {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		flex: 1;
		min-width: 0;
		font-weight: 400;
	}

	.username {
		display: none;
	}
</style>
