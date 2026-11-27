<script module lang="ts">
	const protocols = [
		{
			value: 'oidc',
			label: 'OpenID Connect',
			description: 'Okta, Entra ID, Google Workspace, Keycloak, Auth0 and most modern providers'
		},
		{
			value: 'saml',
			label: 'SAML 2.0',
			description: 'ADFS, Entra ID, Okta, OneLogin, Shibboleth and most enterprise providers'
		}
	];

	const nameIdFormats = [
		{ value: 'email', label: 'Email address' },
		{ value: 'persistent', label: 'Persistent' },
		{ value: 'unspecified', label: 'Unspecified' }
	];
</script>

<script lang="ts">
	import { messageOf, ssoApi, type SSOConnection, type SSOTestResult } from '#lib/api/index.js';
	import { providerAddresses } from './addresses';
	import {
		Alert,
		Button,
		DangerZone,
		FieldGrid,
		FormSection,
		Input,
		PasswordInput,
		Select,
		SwitchField,
		Textarea
	} from '#lib/components/ui/index.js';
	import type { SSOForm } from './form.svelte';

	type Props = {
		form: SSOForm;
		/** The saved connection; null while a new one is being made. */
		current: SSOConnection | null;
		canWrite: boolean;
		onremove: () => void;
		/** Where the provider answers: the base of the addresses given to the provider. */
		issuer: string;
	};

	let { form, current, canWrite, onremove, issuer }: Props = $props();

	/** What the provider is given, shown from the start: registering at the provider comes
	    before its client ID or metadata can be copied here. */
	const addresses = $derived(providerAddresses(issuer, form.identifier));

	const readOnly = $derived(!canWrite);

	let testing = $state(false);
	let tested = $state<SSOTestResult | null>(null);
	let testError = $state('');

	async function test() {
		testing = true;
		testError = '';
		tested = null;

		try {
			tested = await ssoApi.test(form.toTest());
		} catch (err) {
			testError = messageOf(err);
		} finally {
			testing = false;
		}
	}
</script>

{#snippet testButton()}
	<Button variant="subtle" size="sm" onclick={test} loading={testing} disabled={testing}>
		<i class="ri-flask-line" aria-hidden="true"></i>
		Test connection
	</Button>
{/snippet}

{#snippet testResult()}
	{#if testError}
		<Alert>{testError}</Alert>
	{:else if tested?.authorization_endpoint}
		<Alert tone="success">
			{`Discovery worked: people are sent to ${tested.authorization_endpoint} to sign in.`}
		</Alert>
		{#if tested.unsupported_scopes?.length}
			<Alert tone="warning">
				{`The provider does not list ${tested.unsupported_scopes.join(' ')} among its scopes, and some providers — Keycloak among them — refuse the whole sign-in over a scope they do not know. Remove it, or add it at the provider first.`}
			</Alert>
		{/if}
	{:else if tested?.identity_provider}
		<Alert tone="success">
			{`The metadata is ${tested.identity_provider.entity_id}'s: people are sent to ${tested.identity_provider.sso_url} to sign in.`}
		</Alert>
	{/if}
{/snippet}

<FormSection title="Connection" description="What the sign-in page calls it, and whether it is on.">
	{#if !current}
		<Select label="Protocol" bind:value={form.protocol} options={protocols} />
	{/if}

	<FieldGrid>
		<Input
			label="Name"
			bind:value={form.name}
			hint={`What the sign-in page calls it: “Continue with ${form.name.trim() || 'Acme'}”.`}
			required
			{readOnly}
		/>
		<Input
			label="Identifier"
			bind:value={form.slug}
			hint="In the addresses the provider is given, so it cannot change. Made from the name unless you give one."
			placeholder={form.identifier || 'acme'}
			readOnly={current !== null || readOnly}
		/>
	</FieldGrid>

	<SwitchField
		label="On"
		description="Off, nobody signs in through it — set it up and test it first."
		bind:checked={form.enabled}
		disabled={readOnly}
	/>
</FormSection>

{#if form.protocol === 'oidc'}
	<FormSection
		title="OpenID Connect"
		description="Register this server at the provider as a web application with the redirect URI below, then copy its issuer and credentials here."
		action={canWrite ? testButton : undefined}
	>
		<Input
			label="Redirect URI to register at the provider"
			value={form.identifier ? addresses.callback : ''}
			placeholder="Name the connection first"
			hint="The provider sends people back here after they sign in."
			readOnly
			copyable={form.identifier !== ''}
		/>
		<Input
			label="Issuer"
			bind:value={form.issuer}
			hint="Its discovery document is read from the issuer's /.well-known/openid-configuration."
			placeholder="https://acme.okta.com"
			required
			{readOnly}
		/>
		<FieldGrid>
			<Input label="Client ID" bind:value={form.clientId} required {readOnly} />
			<Select
				label="Authenticate with"
				bind:value={form.clientAuth}
				options={[
					{ value: 'client_secret', label: 'Client secret' },
					{ value: 'private_key_jwt', label: 'Signed assertion (private_key_jwt)' }
				]}
				hint={form.clientAuth === 'private_key_jwt'
					? 'This server signs with a key of its own; give the provider its JWKS URL (Service provider tab, once saved).'
					: undefined}
				disabled={readOnly}
			/>
		</FieldGrid>
		{#if form.clientAuth === 'client_secret'}
			<PasswordInput
				label="Client secret"
				bind:value={form.clientSecret}
				autocomplete="new-password"
				hint={current?.has_client_secret
					? 'A secret is stored. Type a new one to replace it.'
					: undefined}
				disabled={readOnly}
			/>
		{/if}
		<Input
			label="Scopes"
			bind:value={form.scopes}
			hint="Space separated. openid is always asked for; email and profile unless you name others."
			placeholder="email profile groups"
			{readOnly}
		/>
		<SwitchField
			label="Trust addresses sent without email_verified"
			description="Off, an address is taken only when the provider’s email_verified says it is verified, which speaks for the email claim alone: an address read from upn or any other claim is refused. Some providers, Microsoft Entra among them, never send it. On, such addresses are taken as proved and may sign in to existing accounts: turn it on only for a provider that owns every address it sends. An address the provider says is not verified is always refused."
			bind:checked={form.assumeEmailVerified}
			disabled={readOnly}
		/>
		{@render testResult()}
	</FormSection>
{:else}
	<FormSection
		title="SAML 2.0"
		description="Give the provider these two, then its own metadata here: from an address, or the file it gave you."
		action={canWrite ? testButton : undefined}
	>
		<FieldGrid>
			<Input
				label="ACS URL (reply URL)"
				value={form.identifier ? addresses.acs : ''}
				placeholder="Name the connection first"
				readOnly
				copyable={form.identifier !== ''}
			/>
			<Input
				label="Entity ID (audience)"
				value={form.identifier ? addresses.entityId : ''}
				placeholder="Name the connection first"
				readOnly
				copyable={form.identifier !== ''}
			/>
		</FieldGrid>
		<Input
			label="Metadata URL"
			bind:value={form.metadataUrl}
			hint="Read when you save, and again with Refresh metadata."
			placeholder="https://login.microsoftonline.com/…/federationmetadata.xml"
			{readOnly}
		/>
		{#if !form.metadataUrl.trim()}
			<Textarea
				label="Metadata XML"
				bind:value={form.metadata}
				rows={5}
				hint="Or paste the file the provider gave you."
			/>
		{/if}
		<FieldGrid>
			<Select
				label="Name ID format"
				bind:value={form.nameIdFormat}
				options={nameIdFormats}
				{readOnly}
			/>
		</FieldGrid>
		<SwitchField
			label="Sign authentication requests"
			description="For providers that require it. The certificate is in this server's metadata."
			bind:checked={form.signRequests}
			disabled={readOnly}
		/>
		{@render testResult()}
	</FormSection>
{/if}

{#if canWrite && current}
	<DangerZone
		title="Remove this connection"
		description="The people who sign in through it keep their accounts, and sign in however else they can."
		onclick={onremove}
	/>
{/if}
