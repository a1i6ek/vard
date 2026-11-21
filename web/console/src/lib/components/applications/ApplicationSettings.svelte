<script lang="ts">
	import type { Application } from '#lib/api/index.js';
	import type { ApplicationForm } from './form.svelte';
	import CredentialsSection from './settings/CredentialsSection.svelte';
	import GeneralSection from './settings/GeneralSection.svelte';
	import GrantsSection from './settings/GrantsSection.svelte';
	import LifetimesSection from './settings/LifetimesSection.svelte';
	import RedirectsSection from './settings/RedirectsSection.svelte';
	import RolesSection from './settings/RolesSection.svelte';
	import SignInSection from './settings/SignInSection.svelte';

	type Props = {
		form: ApplicationForm;
		/** The saved application; null while a new one is being made. */
		current: Application | null;
		editable: boolean;
		saving: boolean;
		onrotate: () => void;
	};

	let { form, current, editable, saving, onrotate }: Props = $props();
</script>

<!-- A machine client has no user: nothing to redirect, no sign-in page, no
     roles. The fieldset is made again when the profile changes: Chrome does not
     lay a fieldset's container-query sections out again when siblings between
     them are removed, and leaves them collapsed. -->
{#key form.input.profile}
	<fieldset disabled={!editable}>
		<GeneralSection {form} {current} />
		<CredentialsSection {form} {current} {editable} {saving} {onrotate} />

		{#if !form.machine}
			<RedirectsSection {form} />
		{/if}

		<GrantsSection {form} />
		<LifetimesSection {form} />

		{#if !form.machine}
			<SignInSection {form} />
			<RolesSection {form} {current} />
		{/if}
	</fieldset>
{/key}

<style lang="scss">
	fieldset {
		min-width: 0;
		margin: 0;
		padding: 0;
		border: none;
	}
</style>
