<script lang="ts">
	import { Toast, Toaster } from '@ark-ui/svelte/toast';
	import { toaster } from './toast';
</script>

<!-- The look is components.css's .toast, the same as the sign-in pages';
     what is here is Ark's placing and motion. -->
<Toaster {toaster}>
	{#snippet children(toast)}
		<Toast.Root class="toast">
			<span class="toast-glyph">
				{#if toast().type === 'success'}
					<i class="ri-checkbox-circle-fill" aria-hidden="true"></i>
				{:else}
					<i class="ri-error-warning-fill" aria-hidden="true"></i>
				{/if}
			</span>
			<div class="toast-message">
				<Toast.Title class="toast-title">{toast().title}</Toast.Title>
				{#if toast().description}
					<Toast.Description class="toast-description">{toast().description}</Toast.Description>
				{/if}
			</div>
			<Toast.CloseTrigger class="toast-close" aria-label="Dismiss">
				<i class="ri-close-line" aria-hidden="true"></i>
			</Toast.CloseTrigger>
		</Toast.Root>
	{/snippet}
</Toaster>

<style>
	:global([data-scope='toast'][data-part='root'].toast) {
		translate: var(--x) var(--y);
		scale: var(--scale);
		z-index: var(--z-index);
		height: var(--height);
		opacity: var(--opacity);
		will-change: translate, opacity, scale;
		transition:
			translate 400ms,
			scale 400ms,
			opacity 400ms,
			height 400ms;
		transition-timing-function: cubic-bezier(0.21, 1.02, 0.73, 1);
	}

	:global([data-scope='toast'][data-part='root'].toast[data-state='closed']) {
		transition:
			translate 200ms,
			scale 200ms,
			opacity 200ms;
		transition-timing-function: cubic-bezier(0.06, 0.71, 0.55, 1);
	}

	@media (prefers-reduced-motion: reduce) {
		:global([data-scope='toast'][data-part='root'].toast),
		:global([data-scope='toast'][data-part='root'].toast[data-state='closed']) {
			transition: opacity 150ms;
		}
	}
</style>
