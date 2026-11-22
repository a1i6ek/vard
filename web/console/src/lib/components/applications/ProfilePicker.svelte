<script lang="ts">
	import type { ApplicationProfile } from '#lib/api/index.js';
	import { profiles } from './applications';

	type Props = {
		value: ApplicationProfile;
		onchange: (profile: ApplicationProfile) => void;
	};

	let { value, onchange }: Props = $props();

	/** What each kind of client is, in the words of whoever is building it. */
	const examples: Record<ApplicationProfile, string> = {
		web: 'A site with its own server: Next.js, Rails, Django, Laravel, ASP.NET.',
		spa: 'A front end that talks to APIs straight from the browser: React, Vue, Svelte.',
		native: 'An iPhone or Android app, React Native or Flutter.',
		desktop: 'A desktop app, a command-line tool, or a TV that signs in with a code.',
		machine: 'A backend job, a service or an agent that calls APIs as itself, with no user.'
	};

	const order = Object.keys(profiles) as ApplicationProfile[];
</script>

<!-- Native radios: the arrow keys move between them, and a screen reader
     says how many there are and which is chosen. -->
<fieldset class="picker">
	<legend>What kind of application is it?</legend>
	<p class="lead">
		The kind decides what it may use: whether it keeps a secret, where it may be sent back to, and
		how it proves itself. It cannot be changed later.
	</p>

	<div class="options">
		{#each order as profile (profile)}
			{@const kind = profiles[profile]}
			<label class="option" class:chosen={value === profile}>
				<input
					type="radio"
					name="profile"
					value={profile}
					checked={value === profile}
					onchange={() => onchange(profile)}
				/>
				<span class="mark" aria-hidden="true"><i class={kind.icon}></i></span>
				<span class="text">
					<strong>{kind.label}</strong>
					<span>{examples[profile]}</span>
					<span class="detail">{kind.description}</span>
				</span>
			</label>
		{/each}
	</div>
</fieldset>

<style lang="scss">
	@use '@web/styles/abstracts' as *;

	.picker {
		display: grid;
		gap: var(--space-3);
		min-width: 0;
		margin: 0;
		padding: 0;
		border: 0;
	}

	legend {
		padding: 0;
		font-size: var(--text-lg);
		font-weight: 600;
	}

	.lead {
		margin: 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.options {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(16rem, 1fr));
		gap: var(--space-3);
	}

	.option {
		position: relative;
		display: flex;
		align-items: flex-start;
		gap: var(--space-3);
		padding: var(--space-3) var(--space-4);
		border: 1px solid var(--color-border);
		border-radius: var(--radius-surface);
		cursor: pointer;
		transition:
			border-color var(--speed),
			background var(--speed);
	}

	.option:hover {
		border-color: var(--color-border-strong);
	}

	.option.chosen {
		border-color: var(--color-primary);
		background: var(--color-brand-subtle);
	}

	.option:has(input:focus-visible) {
		@include focus-ring;
	}

	input {
		@include visually-hidden;
	}

	.mark {
		display: grid;
		flex: none;
		place-items: center;
		width: 36px;
		height: 36px;
		border-radius: var(--radius-md);
		background: var(--color-secondary);
		font-size: var(--text-lg);
	}

	.chosen .mark {
		background: var(--color-primary);
		color: var(--color-primary-text);
	}

	.text {
		display: grid;
		gap: var(--nudge-2);
		min-width: 0;
		font-size: var(--text-sm);
	}

	.text strong {
		font-size: var(--text-base);
	}

	.detail {
		color: var(--color-text-hint);
	}

	@media (prefers-reduced-motion: reduce) {
		.option {
			transition: none;
		}
	}
</style>
