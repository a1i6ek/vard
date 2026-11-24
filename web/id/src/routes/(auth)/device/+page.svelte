<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { ApiError, account } from '#lib/api/index.js';
	import {
		AuthCard,
		Decision,
		DeviceCodeField,
		DeviceOrigin,
		LegalLinkList,
		ScopeList
	} from '#lib/components/auth/index.js';
	import { Alert, Button, Form, Note, PageTitle } from '#lib/components/ui/index.js';
	import { messageOf, useTranslator } from '#lib/i18n/index.js';
	import { legalLinks } from '#lib/utils/legal.js';
	import { secondFactorHref } from '#lib/utils/links.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const t = useTranslator();

	// svelte-ignore state_referenced_locally
	let code = $state(data.code);
	let sending = $state<'yes' | 'no' | null>(null);
	let answered = $state<'allowed' | 'denied' | null>(null);
	let error = $state('');

	// The organization's own links: the step is about its terms, whatever the
	// application has of its own.
	const terms = $derived(
		legalLinks(null, data.organization).map((link) => ({ label: t(link.key), href: link.href }))
	);
	const askTerms = $derived(data.device?.agreements ?? false);

	const loadError = $derived(
		data.problem ? messageOf(new ApiError(data.problem.status, '', data.problem.code), t) : ''
	);

	/** The code goes in the address, so the page loads what the device asks for. */
	async function look() {
		if (code.trim() === '') return;

		error = '';
		const query = new URLSearchParams({ user_code: code.trim() });
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- resolved, with a query
		await goto(`${resolve('/(auth)/device')}?${query}`, { replace: true });
	}

	/** Starts the step-up and goes to finish it; false when there was nothing to start. */
	async function stepUp(): Promise<boolean> {
		const { second_factor } = await account.stepUp(data.code);
		if (!second_factor) return false;

		const back = `${resolve('/(auth)/device')}?${new URLSearchParams({ user_code: data.code })}`;
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- secondFactorHref resolves it
		await goto(secondFactorHref(null, back));
		return true;
	}

	async function answer(yes: boolean) {
		error = '';
		sending = yes ? 'yes' : 'no';

		try {
			await account.answerDevice({ user_code: data.code, yes, accept_terms: yes && askTerms });
			answered = yes ? 'allowed' : 'denied';
		} catch (err) {
			// The application asks for a second factor this session has not
			// passed: pass it (or set one up), then come back to allow it.
			if (err instanceof ApiError && err.code === 'step_up_required' && (await stepUp())) return;
			error = messageOf(err, t);
		} finally {
			sending = null;
		}
	}
</script>

<PageTitle parts={[t('device.head')]} />

{#if answered}
	<AuthCard
		organization={data.organization}
		application={data.device?.application}
		title={answered === 'allowed' ? t('device.allowed_title') : t('device.denied_title')}
	>
		<Alert tone={answered === 'allowed' ? 'success' : 'info'}>
			{answered === 'allowed' ? t('device.allowed_body') : t('device.denied_body')}
		</Alert>
	</AuthCard>
{:else if data.device}
	<AuthCard
		organization={data.organization}
		application={data.device.application}
		title={t('device.title')}
		subtitle={t('device.subtitle', { email: data.user.email })}
	>
		{#if error}<Alert>{error}</Alert>{/if}

		<DeviceOrigin origin={data.device.requested_from} />

		<p class="asks">{t('device.asks', { app: data.device.application.name })}</p>
		<ScopeList scopes={data.device.scopes} />

		{#if askTerms}
			<p>{t('device.agreements', { org: data.organization?.name ?? '' })}</p>
			<LegalLinkList links={terms} />
		{/if}

		<Decision
			yes={askTerms ? t('device.accept_and_allow') : t('device.allow')}
			no={t('device.deny')}
			{sending}
			onanswer={answer}
		/>
	</AuthCard>
{:else}
	<AuthCard
		organization={data.organization}
		title={t('device.title')}
		subtitle={t('device.subtitle', { email: data.user.email })}
	>
		{#if loadError}<Alert>{loadError}</Alert>{/if}

		<Form onsubmit={look}>
			<Note>{t('device.enter')}</Note>
			<DeviceCodeField
				label={t('device.code_label')}
				hint={t('device.code_hint')}
				bind:value={code}
			/>
			<Button type="submit" block disabled={code.replace('-', '').length !== 8}>
				{t('device.continue')}
			</Button>
		</Form>
	</AuthCard>
{/if}

<style>
	.asks {
		margin: 0;
		font-weight: 700;
	}

	p {
		margin: 0;
	}
</style>
