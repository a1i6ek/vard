<script lang="ts">
	import type { APIAccess, Application } from '#lib/api/index.js';
	import { ADMIN_API_IDENTIFIER } from '#lib/brand.js';
	import { Alert, CodeBlock, CopyButton, Skeleton } from '#lib/components/ui/index.js';
	import { exampleScript, tokenEndpoint } from './machine';

	type Props = {
		application: Application;
		/** Every API and what the application may do on it; undefined while loading. */
		access: APIAccess[] | undefined;
		/** Where the admin API answers: this panel's own address and its path. */
		adminApiUrl: string;
	};

	let { application, access, adminApiUrl }: Props = $props();

	/** The APIs it may call, with the scopes its tokens carry for each. */
	const apis = $derived(
		(access ?? [])
			.filter((api) => api.authorized)
			.map((api) => ({
				...api,
				granted: api.scopes.filter((scope) => scope.allowed).map((scope) => scope.name),
				url: api.identifier === ADMIN_API_IDENTIFIER ? adminApiUrl : undefined
			}))
	);

	const issuer = $derived(application.issuer.replace(/\/+$/, ''));
	const hasKey = $derived((application.jwks?.keys.length ?? 0) > 0 || application.jwks_uri !== '');

	type Row = { label: string; value: string; copy: string };

	const rows = $derived<Row[]>([
		{ label: 'Token endpoint', value: tokenEndpoint(issuer), copy: 'token endpoint' },
		{ label: 'Client ID', value: application.client_id, copy: 'client ID' },
		...apis.flatMap((api) => [
			{ label: `Resource · ${api.name}`, value: api.identifier, copy: `${api.name} identifier` },
			...(api.url
				? [{ label: `${api.name} address`, value: api.url, copy: `${api.name} address` }]
				: [])
		])
	]);

	const example = $derived(
		apis[0]
			? exampleScript({
					issuer,
					clientId: application.client_id,
					resource: apis[0].identifier,
					apiUrl: apis[0].url
				})
			: ''
	);
</script>

<section class="setup" aria-labelledby="machine-setup-title">
	<header>
		<h3 id="machine-setup-title">Connect {application.name}</h3>
		<p>
			A service with no user: it asks for a token for one API with its own key, then calls the API
			with that token. Tokens come from {issuer}, the address the sign-in pages and every OAuth
			endpoint answer on{apis.some((api) => api.url)
				? '; the admin API answers on this panel’s address'
				: ''}.
		</p>
	</header>

	{#if !hasKey}
		<Alert tone="warning">
			No key yet: it cannot get tokens. Generate one under Settings, in Credentials.
		</Alert>
	{/if}

	{#if access === undefined}
		<Skeleton lines={3} />
	{:else if apis.length === 0}
		<Alert tone="warning">
			It may call no API yet. Choose an API and the scopes it gets under API access.
		</Alert>
	{:else if apis.some((api) => api.granted.length === 0)}
		<Alert tone="warning">
			It has no scopes on {apis
				.filter((api) => api.granted.length === 0)
				.map((api) => api.name)
				.join(', ')}: its tokens there allow nothing. Choose the scopes it needs under API access.
		</Alert>
	{/if}

	<dl>
		{#each rows as row (row.label)}
			<div class="row">
				<dt>{row.label}</dt>
				<dd>
					<code>{row.value}</code>
					<CopyButton value={row.value} label={row.copy} />
				</dd>
			</div>
		{/each}
		{#each apis as api (api.id)}
			<div class="row">
				<dt>Scopes · {api.name}</dt>
				<dd><code>{api.granted.join(' ') || 'none'}</code></dd>
			</div>
		{/each}
	</dl>

	<div class="steps">
		<h4>How it gets a token</h4>
		<ol>
			<li>
				Sign a short JWT with its key (private_key_jwt): <code>iss</code> and <code>sub</code> are
				the client ID, <code>aud</code> is {issuer}, with a fresh <code>jti</code> and an
				<code>exp</code> a minute away.
			</li>
			<li>
				Post <code>grant_type=client_credentials</code>, the API's identifier as
				<code>resource</code>, and the JWT as <code>client_assertion</code> to the token endpoint,
				with a DPoP proof. The first answer may ask for a nonce (<code>use_dpop_nonce</code>): send
				the proof again with the one in its <code>DPoP-Nonce</code> header.
			</li>
			<li>
				Call the API with <code>Authorization: DPoP &lt;access token&gt;</code> and a fresh proof for
				each request. A token lasts at most fifteen minutes; ask for another the same way.
			</li>
		</ol>
	</div>

	{#if example}
		<CodeBlock
			code={example}
			title="A complete client · Node 20+ or Bun, no dependencies"
			copy="example client"
		/>
	{/if}
</section>

<style>
	.setup {
		display: flex;
		flex-direction: column;
		gap: var(--space-4);
	}

	header h3 {
		margin: 0;
		font-size: var(--text-lg);
	}

	header p {
		margin: var(--space-1) 0 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	dl {
		margin: 0;
		overflow: hidden;
		border: var(--ds-border-width) solid var(--color-border);
		border-radius: var(--radius-box);
	}

	.row {
		display: grid;
		grid-template-columns: minmax(10rem, 14rem) 1fr;
		align-items: center;
		gap: var(--space-3);
		padding: var(--space-2) var(--space-3);
	}

	.row + .row {
		border-top: var(--ds-border-width) solid var(--color-border);
	}

	dt {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	dd {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-2);
		min-width: 0;
		margin: 0;
	}

	dd code {
		overflow-wrap: anywhere;
		font-family: var(--font-mono);
		font-size: var(--text-sm);
	}

	.steps h4 {
		margin: 0 0 var(--space-2);
		font-size: var(--text-base);
	}

	.steps ol {
		display: flex;
		flex-direction: column;
		gap: var(--space-2);
		margin: 0;
		padding-left: var(--space-4);
		font-size: var(--text-sm);
		line-height: var(--leading-md);
	}

	.steps code {
		font-family: var(--font-mono);
		font-size: 0.92em;
	}

	@media (max-width: 36rem) {
		.row {
			grid-template-columns: 1fr;
			gap: var(--nudge-4);
		}
	}
</style>
