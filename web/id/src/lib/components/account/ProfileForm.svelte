<script lang="ts">
	import { refreshAll } from '$app/navigation';
	import { account, type User } from '#lib/api/index.js';
	import { LIMITS } from '#lib/constants.js';
	import { messageOf, useTranslator } from '#lib/i18n/index.js';
	import { notify } from '#lib/state/toast.svelte.js';
	import { Button, FieldRow, Form, Panel, TextField } from '../ui/index.js';
	import EmailChange from './EmailChange.svelte';

	type Props = {
		user: User;
		/** Whether the login flow lets people change their address themselves. */
		mayChangeEmail: boolean;
	};

	let { user, mayChangeEmail }: Props = $props();

	const t = useTranslator();

	let editing = $state(false);
	let changingEmail = $state(false);
	let firstName = $state('');
	let lastName = $state('');
	let saving = $state(false);

	const name = $derived(`${user.first_name} ${user.last_name}`.trim());
	const changed = $derived(
		firstName.trim() !== user.first_name || lastName.trim() !== user.last_name
	);

	// Each edit starts from the saved names.
	function edit() {
		firstName = user.first_name;
		lastName = user.last_name;
		editing = true;
	}

	async function save() {
		if (!changed || saving) return;

		saving = true;

		try {
			await account.updateProfile({ first_name: firstName.trim(), last_name: lastName.trim() });
			notify.success(t('profile.saved'));
			editing = false;
			await refreshAll();
		} catch (err) {
			notify.error(messageOf(err, t));
		} finally {
			saving = false;
		}
	}
</script>

<!-- Read as rows, each changed in place: the row's own button opens its
     fields, and only then does the card carry Save and Cancel. -->
<Form onsubmit={save}>
	<Panel
		title={t('profile.personal_title')}
		description={t('profile.personal_description')}
		footer={editing ? actions : undefined}
	>
		<div class="rows">
			<div class="row">
				<span class="label">{t('profile.name')}</span>
				{#if editing}
					<div class="fields">
						<FieldRow>
							<TextField
								label={t('field.first_name')}
								bind:value={firstName}
								autocomplete="given-name"
								maxlength={LIMITS.name}
								disabled={saving}
							/>
							<TextField
								label={t('field.last_name')}
								bind:value={lastName}
								autocomplete="family-name"
								maxlength={LIMITS.name}
								disabled={saving}
							/>
						</FieldRow>
					</div>
				{:else}
					<span class={['value', { empty: name === '' }]}>{name || t('profile.no_name')}</span>
					<Button variant="subtle" size="sm" icon="ri-pencil-line" onclick={edit}>
						{t('profile.edit')}
					</Button>
				{/if}
			</div>

			<div class="row">
				<span class="label">{t('field.email')}</span>
				<div class="value">
					<span>{user.email}</span>
					<span class="hint">
						{mayChangeEmail ? t('account.email_change_body') : t('profile.email_hint')}
					</span>
				</div>
				{#if mayChangeEmail && !changingEmail}
					<Button variant="subtle" size="sm" onclick={() => (changingEmail = true)}>
						{t('account.email_change')}
					</Button>
				{/if}
			</div>

			{#if changingEmail}
				<div class="fields">
					<EmailChange bind:open={changingEmail} />
				</div>
			{/if}
		</div>
	</Panel>
</Form>

{#snippet actions()}
	<Button variant="subtle" disabled={saving} onclick={() => (editing = false)}>
		{t('action.cancel')}
	</Button>
	<Button type="submit" loading={saving} disabled={!changed}>
		{t('action.save_changes')}
	</Button>
{/snippet}

<style>
	.rows {
		display: flex;
		flex-direction: column;
	}

	.row {
		display: grid;
		grid-template-columns: 9rem minmax(0, 1fr) auto;
		align-items: center;
		gap: var(--space-3);
		min-height: var(--control-height-lg);
		padding: var(--space-3) 0;
	}

	.row + .row {
		border-top: 1px solid var(--color-border);
	}

	/* Open, the row's label stays level with its fields rather than their top. */
	.row:has(.fields) {
		align-items: start;
	}

	.row:has(.fields) > .label {
		line-height: var(--field-height);
	}

	.row .fields {
		grid-column: 2 / -1;
	}

	.label {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
	}

	.value {
		display: flex;
		flex-direction: column;
		gap: var(--nudge-2);
		min-width: 0;
		font-weight: 500;
		overflow-wrap: anywhere;
	}

	.value.empty {
		color: var(--color-text-hint);
		font-weight: 400;
	}

	.hint {
		color: var(--color-text-hint);
		font-size: var(--text-sm);
		font-weight: 400;
	}

	.rows > .fields {
		display: flex;
		flex-direction: column;
		gap: var(--space-3);
		padding-bottom: var(--space-2);
	}

	@media (max-width: 34rem) {
		.row {
			grid-template-columns: minmax(0, 1fr) auto;
			row-gap: var(--space-1);
		}

		.label {
			grid-column: 1 / -1;
		}

		.row .fields {
			grid-column: 1 / -1;
		}
	}
</style>
