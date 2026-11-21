<script lang="ts">
	import { useTranslator } from '#lib/i18n/index.js';
	import FieldRow from './FieldRow.svelte';
	import PasswordField from './PasswordField.svelte';
	import PasswordRules from './PasswordRules.svelte';

	type Props = {
		password: string;
		confirm: string;
		/** "Password" when there is none yet, "New password" when it replaces one. */
		initial?: boolean;
		/** Side by side, where the page is wide enough for it. */
		row?: boolean;
		disabled?: boolean;
	};

	let {
		password = $bindable(''),
		confirm = $bindable(''),
		initial = false,
		row = false,
		disabled = false
	}: Props = $props();

	const t = useTranslator();

	const mismatch = $derived(confirm !== '' && confirm !== password);
	const uid = $props.id();
	const rulesId = `password-rules-${uid}`;
</script>

{#snippet fields()}
	<PasswordField
		label={initial ? t('field.password') : t('field.new_password')}
		bind:value={password}
		autocomplete="new-password"
		{disabled}
		describedBy={rulesId}
	/>
	<PasswordField
		label={initial ? t('field.confirm_password') : t('field.confirm_new_password')}
		bind:value={confirm}
		autocomplete="new-password"
		{disabled}
		describedBy={rulesId}
		error={mismatch ? t('field.password_mismatch') : undefined}
	/>
{/snippet}

{#if row}
	<FieldRow>{@render fields()}</FieldRow>
{:else}
	{@render fields()}
{/if}

<PasswordRules id={rulesId} {password} {confirm} />
