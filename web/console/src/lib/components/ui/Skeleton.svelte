<script lang="ts">
	type Props = {
		/** Lines of text stood in for; the last one shorter, as a paragraph's is. */
		lines?: number;
		/** A block instead of lines: a chart, a picture, a stat's number. */
		height?: string;
		width?: string;
		round?: boolean;
	};

	let { lines = 1, height, width, round = false }: Props = $props();
</script>

<!-- What is loading is announced by the region that holds it; a skeleton is
     only its shape. -->
{#if height}
	<span class="skeleton" data-round={round || undefined} style:height style:width aria-hidden="true"
	></span>
{:else}
	<span class="skeleton-lines" style:width aria-hidden="true">
		{#each { length: lines }, index (index)}
			<span class="skeleton" data-line></span>
		{/each}
	</span>
{/if}
