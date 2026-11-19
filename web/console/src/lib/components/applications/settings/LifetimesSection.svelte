<script lang="ts">
	import { FormSection, Input } from '#lib/components/ui/index.js';
	import type { ApplicationForm } from '../form.svelte';

	type Props = { form: ApplicationForm };

	let { form }: Props = $props();
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
				label="Refresh token, unused"
				bind:value={form.refreshIdleDays}
				type="number"
				min="1"
				suffix="days"
				hint="Ends a sign-in nobody has used for this long."
			/>
		{/if}
	</div>
</FormSection>

<style>
	.triple {
		display: grid;
		grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
		gap: var(--space-3);
	}
</style>
