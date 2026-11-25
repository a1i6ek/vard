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

<FieldBox
	for={inputId}
	{label}
	{hint}
	{error}
	{noteId}
	filled={String(value ?? '') !== ''}
	invalid={Boolean(error)}
	disabled={Boolean(input.disabled)}
	readonly={Boolean(input.readonly)}
>
	<input
		id={inputId}
		data-part="input"
		bind:value
		aria-invalid={error ? 'true' : undefined}
		aria-describedby={error || hint ? noteId : undefined}
		{...input}
	/>
</FieldBox>
