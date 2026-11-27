<script lang="ts">
	import type { Application } from '#lib/api/index.js';
	import {
		Alert,
		Button,
		FormSection,
		Input,
		Note,
		Select,
		Textarea
	} from '#lib/components/ui/index.js';
	import { formatDateTime } from '#lib/utils/format.js';
	import { saveJson } from '#lib/utils/download.js';
	import { authMethods } from '../applications';
	import type { ApplicationForm } from '../form.svelte';
	import { describeKeys, generateKey, keyFile, keyText, readPublicKey } from '../keys';

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

	/** The name of the key file just saved, until the dialog closes. */
	let savedFile = $state('');
	let generating = $state(false);

	// What is in the box is read as it changes; a PEM key is decoded
	// asynchronously, and a reading overtaken by more typing is dropped.
	$effect(() => {
		const text = form.keyText;
		void readPublicKey(text).then((source) => {
			if (form.keyText === text) form.keys = source;
		});
	});

	const keyStatus = $derived.by(() => {
		const source = form.keys;
		if (source.kind === 'jwks') {
			const count = source.jwks.keys.length;
			return `${count} ${count === 1 ? 'key' : 'keys'}: ${describeKeys(source.jwks)}`;
		}
		if (source.kind === 'jwks_uri') return `Published by the application at ${source.uri}`;
		if (source.kind === 'problem') return 'The key below cannot be used yet.';
		return 'No key yet.';
	});

	async function generate() {
		generating = true;
		try {
			const key = await generateKey();
			const clientId = current?.client_id ?? '';
			const name = `${clientId || 'application'}-key.json`;
			saveJson(name, keyFile(key, clientId, current?.issuer ?? ''));
			form.keys = { kind: 'jwks', jwks: { keys: [key.publicKey] } };
			form.keyText = keyText(form.keys);
			savedFile = name;
		} finally {
			generating = false;
		}
	}
</script>

<FormSection
	title="Credentials"
	description={form.kind.public
		? 'A public client cannot keep a secret, so it has none and must use PKCE.'
		: form.machine
			? 'A machine client proves who it is by signing with a key only it holds (private_key_jwt). It has no secret.'
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
		{#if !form.machine}
			<Select
				label="Client authentication"
				bind:value={form.input.token_auth_method}
				options={authMethods}
			/>
		{/if}

		{#if form.usesKeys}
			<div class="key">
				<div class="key-head">
					<div class="key-status">
						<strong>Signing key</strong>
						<span>{keyStatus}</span>
					</div>
					{#if editable}
						<Button size="sm" variant="subtle" onclick={generate} loading={generating}>
							<i class="ri-key-2-line" aria-hidden="true"></i>
							{form.keys.kind === 'none' ? 'Generate a key' : 'Generate a new key'}
						</Button>
					{/if}
				</div>

				{#if savedFile}
					<Alert tone="success">
						<strong>{savedFile}</strong> was saved: it holds the private key, which never leaves
						this browser otherwise. Give it to the application and keep it secret. Save changes to
						register the public half{current && current.jwks
							? ', which replaces the current key'
							: ''}.
					</Alert>
				{:else if form.keys.kind === 'none'}
					<Alert tone="warning">
						It cannot get tokens until it has a key. Generate one here, or paste the public key of
						one the application already has.
					</Alert>
				{/if}

				<Textarea
					label="Public key"
					bind:value={form.keyText}
					rows={4}
					placeholder="-----BEGIN PUBLIC KEY-----  or  https://…/jwks.json"
					hint="A PEM public key, a JWK or JWK Set, or the https address where the application publishes its keys. Never the private key: it stays with the application."
					error={form.keyProblem || undefined}
				/>
			</div>
		{/if}

		{#if current && usesSecret}
			<div class="secret">
				<Input
					label="Client secret"
					value={maskedSecret}
					readOnly
					hint={current.secret_created_at
						? `Created ${formatDateTime(current.secret_created_at)}. Only its last characters are kept.${
								current.previous_secret_until
									? ` The one it replaced works until ${formatDateTime(current.previous_secret_until)}.`
									: ''
							}`
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
	.key {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
	}

	.key-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: var(--space-3);
	}

	.key-status {
		display: flex;
		flex-direction: column;
		gap: var(--nudge-2);
		min-width: 0;
	}

	.key-status span {
		overflow-wrap: anywhere;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

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
		padding-top: var(--nudge-14);
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
