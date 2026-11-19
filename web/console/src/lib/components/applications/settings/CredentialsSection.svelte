<script lang="ts">
	import type { Application } from '#lib/api/index.js';
	import {
		Button,
		FormSection,
		Input,
		Note,
		SegmentedControl,
		Select,
		Textarea
	} from '#lib/components/ui/index.js';
	import { formatDateTime } from '#lib/utils/format.js';
	import { authMethods } from '../applications';
	import type { ApplicationForm } from '../form.svelte';

	type Props = {
		form: ApplicationForm;
		current: Application | null;
		editable: boolean;
		saving: boolean;
		onrotate: () => void;
	};

	let { form, current, editable, saving, onrotate }: Props = $props();

	/** Only the secret's last characters are kept, so that is all there is to show. */
	const maskedSecret = $derived(
		current?.has_secret ? `••••••••••••••••${current.secret_hint}` : 'No secret yet'
	);

	const method = $derived(form.input.token_auth_method);
	const usesSecret = $derived(method === 'client_secret_basic' || method === 'client_secret_post');

	const keySources = [
		{ value: 'jwks', label: 'Key set' },
		{ value: 'jwks_uri', label: 'Address' }
	] as const;
</script>

<FormSection
	title="Credentials"
	description={form.kind.public
		? 'A public client cannot keep a secret, so it has none and must use PKCE.'
		: 'What the application proves who it is with at the token endpoint.'}
>
	{#if current}
		<Input label="Client ID" value={current.client_id} readOnly copyable />
	{:else}
		<Note boxed>
			{usesSecret
				? 'The client ID and secret are made when you create the application.'
				: 'The client ID is made when you create the application.'}
		</Note>
	{/if}

	{#if !form.kind.public}
		{#if form.machine}
			<Note boxed>
				A machine client signs its requests with a key only it holds (private_key_jwt): it has no
				secret.
			</Note>
		{:else}
			<Select
				label="Client authentication"
				bind:value={form.input.token_auth_method}
				options={authMethods}
			/>
		{/if}

		{#if form.usesKeys}
			<SegmentedControl
				label="Where the public keys come from"
				options={keySources}
				value={form.keysFrom}
				onChange={(value) => (form.keysFrom = value)}
			/>
			{#if form.keysFrom === 'jwks'}
				<Textarea
					label="Public keys (JWK Set)"
					bind:value={form.keySet}
					rows={6}
					placeholder={'{"keys": [{"kty": "EC", "crv": "P-256", "x": "…", "y": "…", "kid": "…"}]}'}
					hint="The application signs with the private half; only the public keys go here. ES256, PS256 or RS256."
					error={form.keySet.trim() === '' ? undefined : form.keySetProblem || undefined}
				/>
			{:else}
				<Input
					label="JWKS URI"
					bind:value={form.input.jwks_uri}
					placeholder="https://shop.example.com/.well-known/jwks.json"
					hint="Where the application publishes its public keys. Fetched again at most once a minute for a key not held."
				/>
			{/if}
		{/if}

		{#if current && usesSecret}
			<div class="secret">
				<Input
					label="Client secret"
					value={maskedSecret}
					readOnly
					hint={current.secret_created_at
						? `Created ${formatDateTime(current.secret_created_at)}. Only its last characters are kept.`
						: undefined}
				/>

				{#if editable}
					<div class="rotate">
						<Button size="sm" variant="subtle" onclick={onrotate} disabled={saving}>
							<i class="ri-refresh-line" aria-hidden="true"></i>
							Rotate secret
						</Button>
					</div>
				{/if}
			</div>
		{/if}
	{/if}
</FormSection>

<style>
	.secret {
		display: grid;
		grid-template-columns: 1fr auto;
		align-items: start;
		gap: var(--space-3);
	}

	.secret :global([data-part='input']) {
		font-family: var(--font-mono);
		letter-spacing: 0.02em;
	}

	.rotate {
		display: flex;
		flex-direction: column;
		align-items: flex-end;
		gap: var(--space-2);
		padding-top: 14px;
	}

	@media (max-width: 36rem) {
		.secret {
			grid-template-columns: 1fr;
		}

		.rotate {
			align-items: flex-start;
			padding-top: 0;
		}
	}
</style>
