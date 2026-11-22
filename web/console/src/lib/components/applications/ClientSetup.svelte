<script lang="ts">
	import type { Application } from '#lib/api/index.js';
	import { CodeBlock, CopyButton } from '#lib/components/ui/index.js';
	import { profiles } from './applications';

	type Props = {
		application: Application;
		/** A client secret just made. It exists outside the application this
		    once: only its hash is kept. */
		secret?: string;
	};

	let { application, secret = '' }: Props = $props();

	type Row = { label: string; value: string; copy: string; secret?: boolean };

	const issuer = $derived(application.issuer.replace(/\/+$/, ''));
	const kind = $derived(profiles[application.profile]);
	const signsIn = $derived(application.grant_types.includes('authorization_code'));
	const deviceFlow = $derived(
		application.grant_types.includes('urn:ietf:params:oauth:grant-type:device_code')
	);

	const rows = $derived<Row[]>([
		{ label: 'Issuer', value: issuer, copy: 'issuer' },
		{
			label: 'Discovery document',
			value: `${issuer}/.well-known/openid-configuration`,
			copy: 'discovery document address'
		},
		{ label: 'Client ID', value: application.client_id, copy: 'client ID' },
		...(secret
			? [{ label: 'Client secret', value: secret, copy: 'client secret', secret: true }]
			: []),
		...(signsIn
			? application.redirect_uris.map((uri) => ({
					label: 'Redirect URI',
					value: uri,
					copy: 'redirect URI'
				}))
			: []),
		...(application.scopes.length > 0
			? [{ label: 'Scopes', value: application.scopes.join(' '), copy: 'scopes' }]
			: [])
	]);

	/** What the client has to do that a library will not guess, in the order
	    it meets them. */
	const steps = $derived(
		[
			signsIn &&
				'Send a PKCE code_challenge with the S256 method on every authorization request. It is always required, and there is no setting to turn it off.',
			application.require_pushed_authorization_requests &&
				`Push each authorization request to ${issuer}/oauth2/par first, then send only the request_uri it answers to the authorization endpoint.`,
			deviceFlow &&
				`Start a sign-in on the device at ${issuer}/oauth2/device_authorization, show the code it answers, and poll the token endpoint.`,
			application.profile === 'machine' &&
				'Ask for tokens with grant_type=client_credentials and resource set to the identifier of the API it calls. A token lasts at most fifteen minutes.',
			application.token_auth_method === 'private_key_jwt' &&
				'Authenticate at the token endpoint with a JWT signed by its own key (private_key_jwt). Register the public key under Settings, inline or at a jwks_uri.',
			application.token_auth_method === 'client_secret_basic' &&
				'Authenticate at the token endpoint with the client ID and secret in an HTTP Basic header.',
			application.token_auth_method === 'client_secret_post' &&
				'Authenticate at the token endpoint with client_id and client_secret in the request body.',
			kind.public &&
				'It has no secret: a public client sends its client_id and proves itself with PKCE.',
			application.dpop_bound_access_tokens &&
				'Send a DPoP proof with every token request. Its access tokens are bound to that key, and an API takes them only with a proof.',
			!secret &&
				application.has_secret &&
				'The client secret is shown only when it is made. Lost it? Rotate it under Settings and give the application the new one.'
		].filter((step): step is string => typeof step === 'string')
	);

	const config = $derived(
		JSON.stringify(
			{
				issuer,
				client_id: application.client_id,
				...(secret ? { client_secret: secret } : {}),
				...(signsIn ? { redirect_uris: application.redirect_uris } : {}),
				scope: application.scopes.join(' ')
			},
			null,
			2
		)
	);

	let panel: HTMLElement;

	$effect(() => {
		if (!secret) return;
		const still = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? true;
		panel.scrollIntoView?.({ behavior: still ? 'auto' : 'smooth', block: 'start' });
	});
</script>

<section class="setup" aria-labelledby="setup-title" bind:this={panel}>
	<header>
		<h3 id="setup-title">Connect {application.name}</h3>
		<p>Give the application these. Each has its own copy button.</p>
	</header>

	{#if secret}
		<div class="once" role="status">
			<i class="ri-error-warning-line" aria-hidden="true"></i>
			<div>
				<strong>Copy the client secret now</strong>
				<p>
					It is shown this once: only its hash is kept. If it is lost, rotate it and give the
					application the new one.
				</p>
			</div>
		</div>
	{/if}

	<dl>
		{#each rows as row, index (index)}
			<div class="row" class:secret={row.secret}>
				<dt>{row.label}</dt>
				<dd>
					<code>{row.value}</code>
					<CopyButton value={row.value} label={row.copy} />
				</dd>
			</div>
		{/each}
	</dl>

	{#if steps.length > 0}
		<div class="steps">
			<h4>What the client has to do</h4>
			<ul>
				{#each steps as step (step)}
					<li>{step}</li>
				{/each}
			</ul>
		</div>
	{/if}

	<CodeBlock code={config} title="All of it, as JSON" copy="settings as JSON" />
</section>

<style lang="scss">
	@use '@web/styles/abstracts' as *;

	.setup {
		display: grid;
		gap: var(--space-4);
		scroll-margin-top: var(--space-5);
	}

	h3,
	h4 {
		margin: 0;
		font-size: var(--text-base);
	}

	h4 {
		font-size: var(--text-sm);
	}

	header p {
		margin: var(--nudge-2) 0 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.once {
		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		padding: var(--space-3) var(--space-4);
		border: 1px solid var(--color-success);
		border-radius: var(--radius-surface);
		background: var(--surface-success);
		color: var(--color-success);
	}

	.once i {
		font-size: var(--text-lg);
	}

	.once p {
		margin: var(--nudge-2) 0 0;
		color: var(--color-text);
		font-size: var(--text-sm);
	}

	dl {
		display: grid;
		margin: 0;
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
	}

	.row {
		display: grid;
		grid-template-columns: 11rem minmax(0, 1fr);
		gap: var(--space-3);
		align-items: center;
		padding: var(--space-2) var(--space-3);
	}

	.row + .row {
		border-top: 1px solid var(--color-border);
	}

	.row.secret {
		background: var(--surface-success);
	}

	dt {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	dd {
		display: flex;
		align-items: center;
		gap: var(--space-2);
		min-width: 0;
		margin: 0;
	}

	code {
		flex: 1;
		min-width: 0;
		font-family: var(--font-mono);
		font-size: var(--text-sm);
		overflow-wrap: anywhere;
	}

	.steps ul {
		display: grid;
		gap: var(--space-2);
		margin: var(--space-2) 0 0;
		padding-left: var(--space-5);
		font-size: var(--text-sm);
	}

	@include below('small') {
		.row {
			grid-template-columns: minmax(0, 1fr);
			gap: var(--nudge-4);
		}
	}
</style>
