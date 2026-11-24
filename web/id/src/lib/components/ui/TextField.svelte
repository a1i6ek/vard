<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';
	import FieldBox from './FieldBox.svelte';

	type Props = Omit<HTMLInputAttributes, 'value'> & {
		label: string;
		value: string;
		hint?: string;
		error?: string;
	};

	let { label, value = $bindable(''), hint, error, id, ...input }: Props = $props();

	const uid = $props.id();
	const inputId = $derived(id ?? `field-${uid}`);
	const noteId = $derived(`${inputId}-note`);
</script>

<div class="field">
	<FieldBox
		for={inputId}
		{label}
		filled={String(value ?? '') !== ''}
		invalid={Boolean(error)}
		disabled={Boolean(input.disabled)}
	>
		<input
			id={inputId}
			bind:value
			aria-invalid={error ? 'true' : undefined}
			aria-describedby={error || hint ? noteId : undefined}
			{...input}
		/>
	</FieldBox>
	{#if error}
		<p id={noteId} class="note error">{error}</p>
	{:else if hint}
		<p id={noteId} class="note">{hint}</p>
	{/if}
</div>

<style>
	.field {
		display: flex;
		flex-direction: column;
		gap: var(--space-1);
		min-width: 0;
	}

	.note {
		margin: 0;
		padding: 0 var(--nudge-12);
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.note.error {
		color: var(--color-danger);
	}
</style>
