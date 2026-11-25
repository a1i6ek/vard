<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		checked: boolean;
		disabled?: boolean;
		id?: string;
		name?: string;
		children: Snippet;
	};

	let { checked = $bindable(false), disabled = false, id, name, children }: Props = $props();

	const uid = $props.id();
	const state = $derived(checked ? 'checked' : 'unchecked');
</script>

<!-- A native checkbox wearing the parts Ark gives the panel's, so
     components.css draws both alike; the sign-in pages load no Ark. -->
<label data-scope="checkbox" data-part="root" data-disabled={disabled || undefined}>
	<input id={id ?? `checkbox-${uid}`} {name} type="checkbox" bind:checked {disabled} />
	<span data-scope="checkbox" data-part="control" data-state={state} aria-hidden="true">
		{#if checked}
			<i class="ri-check-line" style:font-size="14px"></i>
		{/if}
	</span>
	<span data-scope="checkbox" data-part="label">{@render children()}</span>
</label>

<style>
	/* The words may run to a second line, a remark under the choice: the box
	   stays level with the first. */
	label[data-part='root'] {
		align-items: flex-start;
		line-height: 1.5;
	}

	label > [data-part='control'] {
		flex-shrink: 0;
	}

	input {
		position: absolute;
		width: 1px;
		height: 1px;
		opacity: 0;
	}

	[data-part='label'] :global(a) {
		font-weight: 700;
	}
</style>
