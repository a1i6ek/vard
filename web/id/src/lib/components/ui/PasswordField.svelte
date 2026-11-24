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

<div class="field">
	<FieldBox for={inputId} {label} filled={value !== ''} invalid={Boolean(error)} {disabled}>
		<input
			id={inputId}
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
				class="reveal"
				onclick={() => (visible = !visible)}
				aria-label={visible ? t('field.hide_password') : t('field.show_password')}
				aria-pressed={visible}
				{disabled}
			>
				<i class={visible ? 'ri-eye-off-line' : 'ri-eye-line'} aria-hidden="true"></i>
			</button>
		{/snippet}
	</FieldBox>

	{#if error || hint || aside}
		<div class="under">
			{#if error}
				<p id={noteId} class="note error">{error}</p>
			{:else if hint}
				<p id={noteId} class="note">{hint}</p>
			{/if}
			{#if aside}
				<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -- built with resolve() by the page -->
				<a href={aside.href}>{aside.label}</a>
			{/if}
		</div>
	{/if}
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		min-width: 0;
	}

	/* What is said about the field, and where to go if it is forgotten, under
	   the box: the label is inside it now. */
	.under {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--space-2);
		padding: 0 var(--nudge-12);
	}

	.under a {
		margin-left: auto;
		font-size: var(--text-sm);
		font-weight: 400;
		white-space: nowrap;
	}

	.note {
		margin: 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.note.error {
		color: var(--color-danger);
	}

	.reveal {
		display: grid;
		place-items: center;
		width: 36px;
		height: 36px;
		border: 0;
		border-radius: var(--radius-control-sm);
		background: transparent;
		color: var(--color-text-hint);
		font-size: var(--icon-size);
		cursor: pointer;
	}

	.reveal:hover:not(:disabled) {
		background: var(--color-field-hover);
		color: var(--color-text);
	}

	.reveal:focus-visible {
		outline: 2px solid var(--color-focus);
		outline-offset: -2px;
	}
</style>
