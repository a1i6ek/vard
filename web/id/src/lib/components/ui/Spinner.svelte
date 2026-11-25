<script lang="ts">
	type Props = {
		/** Said to a screen reader; without one the spinner sits beside words
		    that already say what is happening. */
		label?: string;
		size?: 'sm' | 'md' | 'lg';
	};

	let { label, size = 'sm' }: Props = $props();
</script>

{#if label}
	<span class="spinner" data-size={size} role="status" aria-label={label}></span>
{:else}
	<span class="spinner" data-size={size} aria-hidden="true"></span>
{/if}
