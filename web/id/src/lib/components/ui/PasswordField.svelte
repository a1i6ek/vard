<script lang="ts">
	import { useTranslator } from '#lib/i18n/index.js';
	import FieldBox from './FieldBox.svelte';

	type Props = {
		label: string;
		value: string;
		autocomplete?: 'current-password' | 'new-password';
		disabled?: boolean;
		hint?: string;
		error?: string;
		aside?: { label: string; href: string };
		/** Another element that describes the field, such as the rules a new
		    password has to meet. */
		describedBy?: string;
	};

	let {
		label,
		value = $bindable(''),
		autocomplete = 'current-password',
		disabled = false,
		hint,
		error,
		aside,
		describedBy
	}: Props = $props();

	const t = useTranslator();

	let visible = $state(false);

	const uid = $props.id();
	const inputId = `password-${uid}`;
	const noteId = `${inputId}-note`;
	const described = $derived(
		[error || hint ? noteId : '', describedBy ?? ''].filter(Boolean).join(' ') || undefined
	);
</script>

<FieldBox
	for={inputId}
	{label}
	{hint}
	{error}
	{noteId}
	after={aside ? forgot : undefined}
	filled={value !== ''}
	invalid={Boolean(error)}
	{disabled}
>
	<input
		id={inputId}
		data-part="input"
		type={visible ? 'text' : 'password'}
		bind:value
		{autocomplete}
		{disabled}
		autocapitalize="none"
		spellcheck="false"
		aria-invalid={error ? 'true' : undefined}
		aria-describedby={described}
	/>
	{#snippet end()}
		<button
			type="button"
			data-part="visibility-trigger"
			onclick={() => (visible = !visible)}
			aria-label={visible ? t('field.hide_password') : t('field.show_password')}
			aria-pressed={visible}
			{disabled}
		>
			<i class={visible ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
		</button>
	{/snippet}
</FieldBox>

{#snippet forgot()}
	{#if aside}
		<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- built with resolve() by the page -->
		<a href={aside.href}>{aside.label}</a>
	{/if}
{/snippet}
