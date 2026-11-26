<script lang="ts">
	import { beforeNavigate, goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { useQueryClient } from '@tanstack/svelte-query';
	import { flowsApi, type LoginFlow, type LoginStep } from '#lib/api/index.js';
	import {
		Alert,
		Button,
		ConfirmDialog,
		DangerZone,
		FieldGrid,
		Input,
		PageHeader,
		Panel,
		SaveBar,
		Select,
		SwitchField,
		Tag,
		Textarea,
		notify
	} from '#lib/components/ui/index.js';
	import { keys } from '#lib/query/index.js';
	import { draftOf, freeSlug, problemsOf, slugFrom, withStep, type FlowDraft } from './steps';

	type Props = {
		/** The flow edited; none makes a new one from `initial`. */
		flow: LoginFlow | null;
		initial: FlowDraft;
		/** Identifiers other flows have. */
		taken: string[];
		editable: boolean;
	};

	let { flow, initial, taken, editable }: Props = $props();

	const queryClient = useQueryClient();
	const creating = $derived(flow === null);

	// The form starts from the flow and is the administrator's from then on.
	// svelte-ignore state_referenced_locally
	let draft = $state<FlowDraft>(draftOf(initial));
	// svelte-ignore state_referenced_locally
	let saved = $state(JSON.stringify(draftOf(initial)));
	// A new flow's identifier follows its name until it is typed itself.
	// svelte-ignore state_referenced_locally
	let slugTouched = $state(!creating);

	let saving = $state(false);
	let deleting = $state(false);
	let confirmingDelete = $state(false);

	const dirty = $derived(JSON.stringify(draftOf(draft)) !== saved);
	const problems = $derived(problemsOf(draft));
	const has = (step: LoginStep) => draft.steps.includes(step);

	function toggle(step: LoginStep, on: boolean) {
		draft.steps = withStep(draft.steps, step, on);
		// A requirement nobody could meet goes with the step that meets it.
		if (step === 'totp' && !on) draft.require_second_factor = false;
	}

	function rename(name: string) {
		draft.name = name;
		if (creating && !slugTouched) draft.slug = freeSlug(slugFrom(name), taken);
	}

	const lifetimes: { value: string; label: string }[] = [
		{ value: '1', label: 'One hour' },
		{ value: '8', label: 'Eight hours, a working day' },
		{ value: '24', label: 'One day' },
		{ value: '168', label: 'One week' },
		{ value: '336', label: 'Two weeks' },
		{ value: '720', label: 'Thirty days' },
		{ value: '2160', label: 'Ninety days' }
	];

	// A length set some other way stays a choice rather than vanishing.
	const lifetimeOptions = $derived(
		lifetimes.some((one) => one.value === String(draft.session_lifetime_hours))
			? lifetimes
			: [
					...lifetimes,
					{
						value: String(draft.session_lifetime_hours),
						label: `${draft.session_lifetime_hours} hours`
					}
				]
	);

	async function save(event: SubmitEvent) {
		event.preventDefault();
		if (problems.length > 0 || saving) return;

		saving = true;
		try {
			// The identifier is in every address the flow is known by, so it
			// is only sent when the flow is made.
			const { slug, ...rest } = draftOf(draft);
			const result = creating
				? await flowsApi.create({ ...rest, slug })
				: await flowsApi.update(flow!.id, rest);

			forgetFlows();
			saved = JSON.stringify(draftOf(draft));
			notify.success(creating ? `${result.flow.name} created` : 'Flow saved');

			if (creating) await goto(resolve('/(panel)/flows/[id]', { id: result.flow.id }));
		} catch (err) {
			notify.error(err, 'Could not save this flow');
		} finally {
			saving = false;
		}
	}

	// Dropped rather than marked stale: the list page seeds its query from
	// what its load just read, which a cached list would otherwise win over.
	function forgetFlows() {
		queryClient.removeQueries({ queryKey: keys.flows.all });
	}

	function discard() {
		draft = JSON.parse(saved);
	}

	async function remove() {
		deleting = true;
		try {
			await flowsApi.remove(flow!.id);
			forgetFlows();
			saved = JSON.stringify(draftOf(draft));
			notify.success(`${flow!.name} deleted`);
			await goto(resolve('/(panel)/flows'));
		} catch (err) {
			notify.error(err, 'Could not delete this flow');
		} finally {
			deleting = false;
			confirmingDelete = false;
		}
	}

	// Changes not saved are not lost by a click elsewhere in the panel.
	let leavingFor = $state<URL | null>(null);
	let confirmingLeave = $state(false);

	beforeNavigate(({ to, cancel, type }) => {
		if (!dirty || saving || deleting) return;
		cancel();
		if (type !== 'leave' && to) {
			leavingFor = to.url;
			confirmingLeave = true;
		}
	});

	async function leave() {
		// Saved as it stands, so the navigation it asked about goes through.
		saved = JSON.stringify(draftOf(draft));
		confirmingLeave = false;
		// eslint-disable-next-line svelte/no-navigation-without-resolve -- where the panel was going
		if (leavingFor) await goto(leavingFor);
	}
</script>

<PageHeader
	crumbs={['Dashboard', 'Login flows', creating ? 'New flow' : draft.name || flow!.name]}
	description="How people sign in to the applications that use this flow. Turn on what they may do; everything else is off."
>
	{#snippet actions()}
		{#if !creating && flow!.is_default}<Tag tone="info">Default</Tag>{/if}
	{/snippet}
</PageHeader>

<form onsubmit={save}>
	<fieldset disabled={!editable}>
		<Panel title="About this flow" icon="ri-information-line">
			<FieldGrid>
				<Input
					label="Name"
					value={draft.name}
					oninput={(event) => rename(event.currentTarget.value)}
					maxlength={100}
					required
				/>
				<Input
					label="Identifier"
					bind:value={draft.slug}
					oninput={() => (slugTouched = true)}
					readOnly={!creating}
					maxlength={64}
					hint={creating
						? 'Lower case letters, numbers and dashes. It cannot change later.'
						: 'Set when the flow was made.'}
				/>
			</FieldGrid>
			<Textarea label="Description" bind:value={draft.description} maxlength={500} rows={2} />

			<SwitchField
				label="On"
				description={draft.is_default
					? 'The default flow is always on.'
					: 'Off, its applications fall back to the default flow.'}
				bind:checked={draft.is_enabled}
				disabled={draft.is_default}
			/>
			<SwitchField
				label="Default flow"
				description={!creating && flow!.is_default
					? 'Applications with no flow of their own use this one. Make another flow the default to change it.'
					: 'Applications with no flow of their own use the default.'}
				bind:checked={draft.is_default}
				disabled={!creating && flow!.is_default}
				onChange={(on) => {
					if (on) draft.is_enabled = true;
				}}
			/>
		</Panel>

		<Panel title="Signing in" icon="ri-login-box-line">
			<SwitchField
				label="Allow signing in"
				description="Off, the sign-in pages say signing in is closed, and ask for nothing."
				bind:checked={draft.allow_sign_in}
			/>
			<SwitchField
				label="Password"
				description="Sign in with an email address and a password."
				checked={has('password')}
				onChange={(on) => toggle('password', on)}
			/>
			<SwitchField
				label="Emailed code"
				description="Sign in with a one-time code sent to the address; set on the One-time codes page."
				checked={has('email_code')}
				onChange={(on) => toggle('email_code', on)}
			/>
			<SwitchField
				label="Other accounts"
				description="Offer the providers on the Social sign-in page, such as Google."
				checked={has('social')}
				onChange={(on) => toggle('social', on)}
			/>
			<SwitchField
				label="“Stay signed in”"
				description="Offer to keep the session after the browser closes."
				bind:checked={draft.allow_remember_me}
			/>
		</Panel>

		<Panel title="New accounts" icon="ri-user-add-line">
			<SwitchField
				label="Allow sign-ups"
				description={has('password')
					? 'People can make an account themselves.'
					: 'Needs passwords: a new account is made with one.'}
				bind:checked={draft.allow_registration}
				disabled={!has('password')}
			/>
			<SwitchField
				label="Send a verification email on sign-up"
				description="A link to confirm the address, sent as the account is made."
				bind:checked={draft.verify_email_on_register}
			/>
			<SwitchField
				label="Require a verified address"
				description="An account is not used until its address is confirmed. Recommended: an unproved address may be somebody else’s."
				bind:checked={draft.require_verified_email}
			/>
		</Panel>

		<Panel title="Passwords and addresses" icon="ri-key-2-line">
			<SwitchField
				label="Forgot password"
				description={has('password')
					? 'Offer a link to reset a forgotten password by email.'
					: 'Needs passwords.'}
				bind:checked={draft.allow_password_reset}
				disabled={!has('password')}
			/>
			<SwitchField
				label="Change email address"
				description="Let people change their address on their account page; the new one is confirmed first."
				bind:checked={draft.allow_email_change}
			/>
		</Panel>

		<Panel title="Two-step verification" icon="ri-shield-keyhole-line">
			<SwitchField
				label="Offer two-step verification"
				description="An authenticator app or a passkey, asked at every sign-in of anyone who set one up."
				checked={has('totp')}
				onChange={(on) => toggle('totp', on)}
			/>
			<SwitchField
				label="Require it"
				description={has('totp')
					? 'Everyone must use it. Someone without one sets it up while signing in.'
					: 'Turn on two-step verification first.'}
				bind:checked={draft.require_second_factor}
				disabled={!has('totp')}
			/>
		</Panel>

		<Panel title="Before the application" icon="ri-file-text-line">
			<SwitchField
				label="Agreements"
				description="Ask for the terms and privacy policy to be accepted, and again when they change."
				checked={has('terms')}
				onChange={(on) => toggle('terms', on)}
			/>
			<SwitchField
				label="Consent screen"
				description="Show what the application asks for and wait to be allowed, once for each new thing."
				checked={has('consent')}
				onChange={(on) => toggle('consent', on)}
			/>
		</Panel>

		<Panel title="Sessions" icon="ri-time-line">
			<Select
				label="Stay signed in for"
				value={String(draft.session_lifetime_hours)}
				options={lifetimeOptions}
				onChange={(value) => (draft.session_lifetime_hours = Number(value))}
				hint="How long a sign-in lasts before it is asked for again."
			/>
		</Panel>
	</fieldset>

	{#if editable && dirty && problems.length > 0}
		<Alert tone="warning">
			{#each problems as problem (problem)}<span class="problem">{problem}</span>{/each}
		</Alert>
	{/if}

	{#if editable && !creating && !flow!.is_default}
		<DangerZone
			title="Delete this flow"
			description="Its applications fall back to the default flow."
			label="Delete flow"
			onclick={() => (confirmingDelete = true)}
		/>
	{/if}

	{#if editable && creating}
		<div class="create">
			<Button variant="subtle" onclick={() => goto(resolve('/(panel)/flows'))}>Cancel</Button>
			<Button type="submit" loading={saving} disabled={problems.length > 0}>Create flow</Button>
		</div>
	{:else if editable && dirty}
		<SaveBar {saving} ready={problems.length === 0} ondiscard={discard} />
	{/if}
</form>

{#if !creating}
	<ConfirmDialog
		bind:open={confirmingDelete}
		title="Delete {flow!.name}?"
		description="Its applications fall back to the default flow. This cannot be undone."
		tone="danger"
		confirmLabel="Delete flow"
		confirmText={flow!.name}
		busy={deleting}
		onConfirm={remove}
	/>
{/if}

<ConfirmDialog
	bind:open={confirmingLeave}
	title="Leave without saving?"
	description="The changes made to this flow are lost."
	confirmLabel="Leave"
	cancelLabel="Stay"
	onConfirm={leave}
/>

<style>
	form,
	fieldset {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	fieldset {
		margin: 0;
		padding: 0;
		border: 0;
	}

	.problem {
		display: block;
	}

	.create {
		display: flex;
		justify-content: flex-end;
		gap: var(--space-2);
	}
</style>
