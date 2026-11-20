<script lang="ts">
	import { FormSection, SwitchField } from '#lib/components/ui/index.js';
	import Choice from '#lib/components/roles/Choice.svelte';
	import { grants, scopes } from '../applications';
	import type { ApplicationForm } from '../form.svelte';

	type Props = { form: ApplicationForm };

	let { form }: Props = $props();

	/** An SPA's, a mobile app's and a machine's tokens are always bound to a key. */
	const bound = $derived(['spa', 'native', 'machine'].includes(form.input.profile));

	/** A public client has no secret to prove itself with, so it cannot act for itself. */
	const offeredGrants = $derived(
		grants.filter((grant) => grant.value !== 'client_credentials' || !form.kind.public)
	);
</script>

{#if form.machine}
	<FormSection
		title="Grant"
		description="A machine client uses client_credentials only: it has no user to sign in, so nothing to redirect to."
	>
		<Choice
			name="client_credentials"
			description="Act as the application itself, with no user"
			checked={true}
			onChange={() => {}}
			disabled
		/>
	</FormSection>
{:else}
	<FormSection
		title="Grants and scopes"
		description="How the application signs users in, and what it may ask for."
	>
		<div class="options">
			<span class="options-label">Grant types</span>
			<div class="grid">
				{#each offeredGrants as grant (grant.value)}
					<Choice
						name={grant.value}
						description={grant.description}
						checked={form.input.grant_types.includes(grant.value)}
						onChange={(on) => form.setGrant(grant.value, on)}
					/>
				{/each}
			</div>
		</div>

		<div class="options">
			<span class="options-label">Scopes</span>
			<div class="grid">
				{#each scopes as scope (scope.value)}
					<Choice
						name={scope.value}
						description={scope.description}
						checked={form.input.scopes.includes(scope.value)}
						onChange={(on) => form.setScope(scope.value, on)}
					/>
				{/each}
			</div>
		</div>

		<SwitchField
			label="Bind tokens to a key (DPoP)"
			description={bound
				? 'Always on for this profile: its tokens are useless to anyone without its key.'
				: 'Refuse a token request without a DPoP proof, so every token is bound to the key that asked for it.'}
			bind:checked={form.input.dpop_bound_access_tokens}
			disabled={bound}
		/>

		<SwitchField
			label="Require pushed requests"
			description="Only accept sign-ins the application pushed to the server first (PAR), so their parameters never travel in the browser's address."
			bind:checked={form.input.require_pushed_authorization_requests}
		/>
	</FormSection>
{/if}

<style>
	.options {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
	}

	.options-label {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		font-weight: 700;
	}

	.grid {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
		gap: 2px var(--space-3);
	}
</style>
