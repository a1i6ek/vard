<script lang="ts">
	import { FormSection, Input, Select } from '#lib/components/ui/index.js';
	import type { ApplicationForm } from '../form.svelte';

	type Props = { form: ApplicationForm };

	let { form }: Props = $props();

	const algorithms = [
		{ value: 'RS256', label: 'RS256', description: 'What every OpenID Connect client accepts' },
		{ value: 'PS256', label: 'PS256', description: 'RSA with PSS padding' },
		{ value: 'ES256', label: 'ES256', description: 'P-256: smaller tokens' }
	];
</script>

<FormSection
	title="Token lifetimes"
	description="How long each token the application is given stays valid."
>
	<div class="triple">
		<Input
			label="Access token"
			bind:value={form.accessMinutes}
			type="number"
			min="1"
			max="60"
			suffix="minutes"
		/>
		{#if !form.machine}
			<Input label="ID token" bind:value={form.idMinutes} type="number" min="1" suffix="minutes" />
			<Input
				label="Refresh token"
				bind:value={form.refreshDays}
				type="number"
				min="1"
				suffix="days"
				hint="However much it is used."
			/>
			<Input
				label="Unused for"
				bind:value={form.refreshIdleDays}
				type="number"
				min="1"
				suffix="days"
				hint="Ends a refresh token nobody has used for this long."
			/>
		{/if}
	</div>
	{#if !form.machine}
		<Select
			label="ID tokens signed with"
			bind:value={form.input.id_token_signed_response_alg}
			options={algorithms}
			hint="Its logout tokens are signed the same way."
		/>
	{/if}
</FormSection>

<style>
	.triple {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
		gap: var(--space-3);
	}
</style>
