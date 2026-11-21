<script module lang="ts">
	import type { Tab } from '#lib/components/ui/index.js';

	const tabs: Tab[] = [
		{ value: 'settings', label: 'Settings', icon: 'ri-settings-3-line' },
		{ value: 'apis', label: 'API access', icon: 'ri-code-box-line' },
		{ value: 'logouts', label: 'Logouts', icon: 'ri-logout-box-r-line' },
		{ value: 'preview', label: 'Token preview', icon: 'ri-shield-check-line' }
	];
</script>

<script lang="ts">
	import { submit } from '#lib/forms.js';
	import { createMutation, useQueryClient } from '@tanstack/svelte-query';
	import {
		applicationsApi,
		type Admin,
		type Application,
		type ApplicationWithSecret
	} from '#lib/api/index.js';
	import {
		Button,
		ConfirmDialog,
		Select,
		FullscreenDialog,
		Tabs,
		notify
	} from '#lib/components/ui/index.js';
	import { keys } from '#lib/query/index.js';
	import ApplicationApis from './ApplicationApis.svelte';
	import LogoutDeliveries from './LogoutDeliveries.svelte';
	import ApplicationSettings from './ApplicationSettings.svelte';
	import ApplicationSummary from './ApplicationSummary.svelte';
	import { ApplicationForm } from './form.svelte';
	import SecretPanel from './SecretPanel.svelte';
	import TokenPreviewPanel from './TokenPreviewPanel.svelte';

	type Props = {
		application: Application | null;
		editable: boolean;
		admin: Admin;
		open: boolean;
	};

	let { application, editable, admin, open = $bindable(false) }: Props = $props();

	const queryClient = useQueryClient();

	const form = new ApplicationForm();

	/** The application as last saved; null until a new one is created. */
	let current = $state<Application | null>(null);
	let tab = $state('settings');
	/** A client secret just made: shown once, here, and never again. */
	let secret = $state('');
	let saving = $state(false);
	let confirmingRotate = $state(false);
	/** How long the secret being replaced keeps working, in seconds. */
	let overlap = $state('0');
	const overlaps = [
		{ value: '0', label: 'Stops at once' },
		{ value: String(60 * 60), label: 'Keeps working for an hour' },
		{ value: String(24 * 60 * 60), label: 'Keeps working for a day' },
		{ value: String(7 * 24 * 60 * 60), label: 'Keeps working for a week' }
	];

	const saveable = $derived(editable && tab === 'settings');

	$effect(() => {
		if (!open) {
			tab = 'settings';
			return;
		}

		current = application;
		secret = '';
		confirmingRotate = false;
		form.fill(application);
	});

	async function saved(result: ApplicationWithSecret) {
		await queryClient.invalidateQueries({ queryKey: keys.applications.all });

		current = result.application;
		form.fill(result.application);

		if (result.client_secret) {
			secret = result.client_secret;
		} else {
			open = false;
		}
	}

	const save = submit<Awaited<ReturnType<typeof applicationsApi.update>>>({
		input: () => (current ? [current.id, form.toInput()] : [form.toInput()]),
		when: () => !(!saveable || !form.ready),
		pending: (busy) => (saving = busy),
		done: (result: ApplicationWithSecret) => {
			notify.success(
				current ? 'Application saved' : 'Application created',
				result.client_secret ? 'Copy its secret now: it is shown once.' : undefined
			);
			return saved(result);
		},
		failed: (err) => notify.error(err, 'Could not save this application')
	});

	const rotate = createMutation(() => ({
		mutationFn: () => applicationsApi.rotateSecret(current!.id, Number(overlap)),
		onSuccess: async (result: ApplicationWithSecret) => {
			notify.success('Secret rotated', 'Copy the new one now: it is shown once.');
			confirmingRotate = false;
			await saved(result);
		},
		onError: (err: unknown) => {
			confirmingRotate = false;
			notify.error(err, 'Could not rotate the secret');
		},
		onSettled: () => {
			saving = false;
		}
	}));

	function rotateNow() {
		if (!current || saving) return;

		saving = true;
		rotate.mutate();
	}
</script>

{#snippet settings()}
	<ApplicationSettings
		{form}
		{current}
		{editable}
		{saving}
		onrotate={() => (confirmingRotate = true)}
	/>
{/snippet}

{#snippet actions()}
	{#if secret}
		<Button onclick={() => (open = false)}>Done</Button>
	{:else if saveable}
		<Button variant="subtle" onclick={() => (open = false)} disabled={saving}>Cancel</Button>
		<Button type="submit" loading={saving} disabled={saving || !form.ready}>
			{saving ? 'Saving…' : current ? 'Save changes' : 'Create application'}
		</Button>
	{/if}
{/snippet}

<FullscreenDialog
	bind:open
	actions={secret || saveable ? actions : undefined}
	title={current ? current.name : 'New application'}
	meta={current?.client_id}
	action={current ? '/actions/applications?/update' : '/actions/applications?/create'}
	onsubmit={save}
>
	{#if current}
		{#if secret}
			<SecretPanel clientId={current.client_id} {secret} />
		{/if}

		<ApplicationSummary application={current} />

		<Tabs {tabs} bind:value={tab} label="Application sections">
			{#snippet panel(value)}
				{#if value === 'settings'}
					{@render settings()}
				{:else if value === 'apis'}
					<ApplicationApis applicationId={current!.id} {editable} />
				{:else if value === 'logouts'}
					<LogoutDeliveries application={current!} />
				{:else}
					<TokenPreviewPanel application={current!} {admin} />
				{/if}
			{/snippet}
		</Tabs>
	{:else}
		{@render settings()}
	{/if}
</FullscreenDialog>

<ConfirmDialog
	bind:open={confirmingRotate}
	title="Rotate the client secret?"
	description="A new secret is made and shown once. Let the current one keep working for a while to give the new one to the application first; once it stops, anything still using it can no longer ask for tokens."
	tone="warning"
	icon="ri-refresh-line"
	confirmLabel="Rotate now"
	busy={saving}
	onConfirm={rotateNow}
>
	<Select label="The current secret" bind:value={overlap} options={overlaps} />
</ConfirmDialog>
