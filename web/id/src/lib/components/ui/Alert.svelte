<script lang="ts">
	import type { Snippet } from 'svelte';

	type Props = {
		tone?: 'danger' | 'warning' | 'info' | 'success';
		children: Snippet;
	};

	let { tone = 'danger', children }: Props = $props();

	const icons = {
		danger: 'ri-error-warning-line',
		warning: 'ri-error-warning-line',
		info: 'ri-information-line',
		success: 'ri-checkbox-circle-line'
	};
</script>

<!-- An error is announced as soon as it appears; a notice is just read in its
     place. -->
<div class="alert {tone}" role={tone === 'danger' ? 'alert' : 'status'}>
	<i class={icons[tone]} aria-hidden="true"></i>
	<div class="body">{@render children()}</div>
</div>

<style>
	.alert {
		--tone: var(--color-danger);
		--tone-surface: var(--surface-danger);

		display: flex;
		align-items: flex-start;
		gap: var(--space-2);
		padding: var(--space-3);
		border: 1px solid color-mix(in srgb, var(--tone), transparent 75%);
		border-radius: var(--radius-box);
		background: var(--tone-surface);
		color: var(--color-text);
		font-size: var(--text-base);
		line-height: 1.5;
	}

	.alert > i {
		flex-shrink: 0;
		margin-top: var(--nudge-1);
		color: var(--tone);
	}

	.body {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.danger {
		color: var(--color-danger);
	}

	.warning {
		--tone: var(--color-warning);
		--tone-surface: var(--surface-warning);
	}

	.info {
		--tone: var(--color-info);
		--tone-surface: var(--surface-info);
	}

	.success {
		--tone: var(--color-success);
		--tone-surface: var(--surface-success);
	}
</style>
