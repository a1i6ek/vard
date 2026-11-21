<script lang="ts">
	import { LIMITS } from '#lib/constants.js';
	import { useTranslator } from '#lib/i18n/index.js';
	import { longEnough } from '#lib/utils/password.js';

	type Props = {
		id: string;
		password: string;
		confirm: string;
	};

	let { id, password, confirm }: Props = $props();

	const t = useTranslator();

	// The longest a password may be is shown only once it is passed: a
	// rule met from the first keystroke is noise.
	const rules = $derived([
		{
			key: 'length',
			met: longEnough(password),
			text: t('field.rule_length', { count: LIMITS.passwordMin })
		},
		...[
			{
				key: 'max',
				met: [...password].length <= LIMITS.passwordMax,
				text: t('field.rule_max', { count: LIMITS.passwordMax })
			}
		].filter((rule) => !rule.met),
		{ key: 'match', met: password !== '' && confirm === password, text: t('field.rule_match') }
	]);
</script>

<!-- What a new password has to be, met or not as it is typed. The field
     points here (aria-describedby), so a screen reader reads the rules with
     it; each says whether it is met in words, not only by its icon. -->
<ul {id} class="rules">
	{#each rules as rule (rule.key)}
		<li class:met={rule.met}>
			<i class={rule.met ? 'ri-checkbox-circle-fill' : 'ri-circle-line'} aria-hidden="true"></i>
			<span>{rule.text}</span>
			<span class="visually-hidden">{rule.met ? t('field.rule_met') : t('field.rule_not_met')}</span
			>
		</li>
	{/each}
</ul>

<style lang="scss">
	@use '@web/styles/abstracts' as *;

	.rules {
		display: grid;
		gap: var(--nudge-4);
		margin: 0;
		padding: 0;
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		list-style: none;
	}

	li {
		display: flex;
		align-items: center;
		gap: var(--space-2);
	}

	li i {
		font-size: var(--text-lg);
	}

	.met {
		color: var(--color-text);
	}

	.met i {
		color: var(--color-success);
	}

	.visually-hidden {
		@include visually-hidden;
	}
</style>
