<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		/** The control's id, which the label names. */
		for: string;
		label: string;
		/** The control holds a value, so the label stays raised. */
		filled: boolean;
		invalid?: boolean;
		disabled?: boolean;
		readonly?: boolean;
		/** Said under the box: an error, or else a hint. */
		error?: string;
		hint?: string;
		/** The id of what is said under the box, for aria-describedby. */
		noteId?: string;
		/** The control: an input wearing data-part="input". */
		children: Snippet;
		/** Inside the box at its right end, such as a password's eye. */
		end?: Snippet;
		/** Under the box, after the note, such as a "forgot password?" link. */
		after?: Snippet;
	};

	let {
		for: control,
		label,
		filled,
		invalid = false,
		disabled = false,
		readonly = false,
		error,
		hint,
		noteId,
		children,
		end,
		after
	}: Props = $props();
</script>

<!-- The same markup and state attributes Ark gives the panel's fields, so
     components.css draws both apps' fields alike, with no script beyond the
     input itself. -->
<div
	class="field-root"
	data-invalid={invalid || undefined}
	data-disabled={disabled || undefined}
	data-readonly={readonly || undefined}
>
	<div class="field-box" data-float={filled || undefined}>
		<label data-part="label" for={control}>{label}</label>
		{@render children()}
		{@render end?.()}
	</div>

	{#if error || hint || after}
		<div class="under">
			{#if error}
				<p id={noteId} data-part="error-text">{error}</p>
			{:else if hint}
				<p id={noteId} data-part="helper-text">{hint}</p>
			{/if}
			{@render after?.()}
		</div>
	{/if}
</div>

<style>
	/* The note and what follows it share the line under the box. */
	.under {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--space-2);
	}

	.under > p {
		margin: 0;
		padding: 0 var(--nudge-12);
		font-size: var(--text-xs);
		line-height: 1.45;
	}

	[data-part='helper-text'] {
		color: var(--color-text-hint);
	}

	[data-part='error-text'] {
		color: var(--color-danger);
	}

	.under > :global(a) {
		margin-left: auto;
		padding-right: var(--nudge-12);
		font-size: var(--text-sm);
		white-space: nowrap;
	}
</style>
