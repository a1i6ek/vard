<script lang="ts">
	import { goto } from '$app/navigation';
	import {
		ApiError,
		requiredSSO,
		signIn,
		type CodeChallenge,
		type SignedIn
	} from '#lib/api/index.js';
	import { AuthCard, CodeForm, SocialButtons } from '#lib/components/auth/index.js';
	import {
		Alert,
		Button,
		Checkbox,
		FieldRow,
		Form,
		NewPasswordFields,
		PageTitle,
		TextField
	} from '#lib/components/ui/index.js';
	import { LIMITS } from '#lib/constants.js';
	import { messageOf, useTranslator } from '#lib/i18n/index.js';
	import { legalLinks } from '#lib/utils/legal.js';
	import { authHref, leaveTo, secondFactorHref, ssoHref } from '#lib/utils/links.js';
	import { newPasswordReady } from '#lib/utils/password.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const t = useTranslator();

	const app = $derived(data.application);

	// Within an application's sign-in, that application and its flow decide;
	// on the account's own sign-in page, the default flow does. A new
	// account is made with a password, so a flow without one takes none.
	const open = $derived(
		data.login.allow_registration &&
			data.login.steps.includes('password') &&
			(app === null || app.allow_registration)
	);

	const agreements = $derived(legalLinks(app, data.organization));
	const needsConsent = $derived(agreements.length > 0);

	let firstName = $state('');
	let lastName = $state('');
	// svelte-ignore state_referenced_locally
	let email = $state(data.signInRequest?.login_hint ?? '');
	let password = $state('');
	let confirm = $state('');
	let accepted = $state(false);
	let error = $state('');
	let sent = $state('');
	let submitting = $state(false);
	let challenge = $state<CodeChallenge | null>(null);

	const canSubmit = $derived(
		email.trim() !== '' &&
			newPasswordReady(password, confirm) &&
			(!needsConsent || accepted) &&
			!submitting
	);

	async function submit() {
		if (!canSubmit || data.expired) return;

		error = '';
		submitting = true;

		try {
			const result = await signIn.register({
				request: data.request ?? '',
				email: email.trim(),
				password,
				first_name: firstName.trim(),
				last_name: lastName.trim(),
				accept_terms: accepted,
				remember: data.login.allow_remember_me
			});

			await arrive(result);
		} catch (err) {
			const sso = requiredSSO(err);
			if (sso) {
				leaveTo(ssoHref(sso.slug, { request: data.request, email: email.trim() }));
				return;
			}

			if (err instanceof ApiError && err.code === 'email_not_verified') {
				sent = messageOf(err, t);
			} else {
				error = messageOf(err, t);
			}
			submitting = false;
		}
	}

	// Where a new account goes next, as the sign-in page sends a sign-in: a
	// second factor the flow requires, an emailed code, the application, or
	// the account itself.
	async function arrive(result: SignedIn) {
		if (result.second_factor) {
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved, with a query
			await goto(secondFactorHref(data.request, data.next), { replace: true });
			return;
		}

		if (result.code) {
			challenge = result.code;
			submitting = false;
			return;
		}

		if (result.redirect_to) {
			leaveTo(result.redirect_to);
			return;
		}

		try {
			// eslint-disable-next-line svelte/no-navigation-without-resolve -- a checked path on this site
			await goto(data.next, { refreshAll: true, replace: true });
		} catch {
			window.location.replace(data.next);
		}
	}
</script>

<PageTitle parts={[t('register.head'), app?.name]} />

{#if sent}
	<AuthCard organization={data.organization} application={app} title={t('verify.title')}>
		<Alert tone="info">{sent}</Alert>

		{#snippet below()}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={authHref('/login', data.request)}>{t('action.back_to_sign_in')}</a>
		{/snippet}
	</AuthCard>
{:else if challenge}
	<AuthCard
		organization={data.organization}
		application={app}
		title={t('login.code_title')}
		subtitle={t('login.code_subtitle', { email: challenge.email })}
	>
		<CodeForm
			{challenge}
			onsignedin={(result) => void arrive(result)}
			onexpired={(message) => {
				challenge = null;
				error = message;
			}}
		/>
	</AuthCard>
{:else if data.expired}
	<AuthCard organization={data.organization} title={t('login.expired_title')}>
		<Alert tone="info">{t('login.expired_body')}</Alert>
	</AuthCard>
{:else if !open}
	<AuthCard organization={data.organization} application={app} title={t('register.closed_title')}>
		<Alert tone="info">
			{app ? t('register.closed_body', { app: app.name }) : t('register.closed_body_here')}
		</Alert>

		{#snippet below()}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={authHref('/login', data.request)}>{t('action.back_to_sign_in')}</a>
		{/snippet}
	</AuthCard>
{:else}
	<AuthCard
		organization={data.organization}
		application={app}
		title={t('register.title')}
		subtitle={app ? t('register.subtitle_app', { app: app.name }) : undefined}
	>
		<Form onsubmit={submit}>
			{#if error}<Alert>{error}</Alert>{/if}

			<FieldRow>
				<TextField
					label={t('field.first_name')}
					bind:value={firstName}
					autocomplete="given-name"
					maxlength={LIMITS.name}
					disabled={submitting}
				/>
				<TextField
					label={t('field.last_name')}
					bind:value={lastName}
					autocomplete="family-name"
					maxlength={LIMITS.name}
					disabled={submitting}
				/>
			</FieldRow>

			<TextField
				label={t('field.email')}
				bind:value={email}
				type="email"
				autocomplete="email"
				autocapitalize="none"
				spellcheck={false}
				maxlength={LIMITS.email}
				disabled={submitting}
			/>

			<NewPasswordFields bind:password bind:confirm initial disabled={submitting} />

			{#if needsConsent}
				<Checkbox bind:checked={accepted} disabled={submitting}>
					{t('register.consent')}
					{#each agreements as agreement, index (agreement.key)}
						{#if index > 0}{` ${t('register.consent_and')} `}{/if}
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a href={agreement.href} target="_blank" rel="noopener noreferrer">
							{t(agreement.key)}
						</a>
					{/each}
				</Checkbox>
			{/if}

			<Button type="submit" block loading={submitting} disabled={!canSubmit}>
				{submitting ? t('register.submitting') : t('register.submit')}
			</Button>
		</Form>

		<SocialButtons
			providers={data.socialProviders}
			request={data.request}
			label={t('register.social')}
			disabled={submitting}
		/>

		{#snippet below()}
			{t('register.have_account')}
			<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
			<a href={authHref('/login', data.request)}>{t('action.sign_in')}</a>
		{/snippet}
	</AuthCard>
{/if}
