<script lang="ts">
	import { fly } from 'svelte/transition';
	import { useTranslator } from '#lib/i18n/index.js';
	import { dismiss, pause, resume, toasts, type ToastTone } from '#lib/state/toast.svelte.js';
	import type { IconName } from './icon';

	const t = useTranslator();

	const icons: Record<ToastTone, IconName> = {
		success: 'ri-checkbox-circle-fill',
		danger: 'ri-error-warning-fill'
	};

	const motion =
		typeof window !== 'undefined' && matchMedia('(prefers-reduced-motion: reduce)').matches
			? { y: 0, duration: 120 }
			: { y: 16, duration: 220 };
</script>

<!-- The look is components.css's .toast, the same as the panel's; what is
     here is the stack and its motion. -->
<section class="toaster" aria-live="polite" aria-label={t('toast.region')}>
	{#each toasts as toast (toast.id)}
		<div
			class="toast"
			role={toast.tone === 'danger' ? 'alert' : 'status'}
			in:fly={motion}
			out:fly={{ ...motion, y: 8 }}
			onmouseenter={() => pause(toast.id)}
			onmouseleave={() => resume(toast.id)}
			onfocusin={() => pause(toast.id)}
			onfocusout={() => resume(toast.id)}
		>
			<span class="toast-glyph"><i class={icons[toast.tone]} aria-hidden="true"></i></span>
			<p class="toast-message">
				<span class="toast-title">{toast.title}</span>
				{#if toast.description}
					<span class="toast-description">{toast.description}</span>
				{/if}
			</p>
			<button
				type="button"
				class="toast-close"
				aria-label={t('action.dismiss')}
				onclick={() => dismiss(toast.id)}
			>
				<i class="ri-close-line" aria-hidden="true"></i>
			</button>
		</div>
	{/each}
</section>

<style>
	.toaster {
		position: fixed;
		bottom: var(--space-4);
		left: 50%;
		z-index: 100;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: var(--space-2);
		width: min(30rem, calc(100vw - 2 * var(--space-4)));
		transform: translateX(-50%);
		pointer-events: none;
	}

	.toast {
		pointer-events: auto;
	}
</style>
