// @vitest-environment jsdom
import { createRawSnippet } from 'svelte';
import { describe, it } from 'vitest';
import { expectAccessible } from '#lib/testing/a11y.js';
import AccountSummary from './account/AccountSummary.svelte';
import AuthenticatorSetup from './account/AuthenticatorSetup.svelte';
import ProfileIntro from './account/ProfileIntro.svelte';
import Decision from './auth/Decision.svelte';
import DeviceCodeField from './auth/DeviceCodeField.svelte';
import DeviceOrigin from './auth/DeviceOrigin.svelte';
import LegalLinkList from './auth/LegalLinkList.svelte';
import PasskeyAdd from './account/PasskeyAdd.svelte';
import PresenceCheck from './account/PresenceCheck.svelte';
import RecoveryCodeSheet from './account/RecoveryCodeSheet.svelte';
import SessionList from './account/SessionList.svelte';
import Alert from './ui/Alert.svelte';
import Button from './ui/Button.svelte';
import Checkbox from './ui/Checkbox.svelte';
import Panel from './ui/Panel.svelte';
import Badge from './ui/Badge.svelte';
import LinkButton from './ui/LinkButton.svelte';
import Note from './ui/Note.svelte';
import Skeleton from './ui/Skeleton.svelte';
import Spinner from './ui/Spinner.svelte';
import NewPasswordFields from './ui/NewPasswordFields.svelte';
import PasswordField from './ui/PasswordField.svelte';
import PasswordRules from './ui/PasswordRules.svelte';
import TextField from './ui/TextField.svelte';

const text = (words: string) => createRawSnippet(() => ({ render: () => `<span>${words}</span>` }));
const nothing = () => {};

describe('the sign-in pages are usable without seeing them', () => {
	it('a text field', () => expectAccessible(TextField, { label: 'Email', value: '' }));

	it('a text field with a hint and an error', () =>
		expectAccessible(TextField, {
			label: 'Code',
			value: '12',
			hint: 'Six digits',
			error: 'That code is not right'
		}));

	it('a password field', () =>
		expectAccessible(PasswordField, { label: 'Password', value: 'secret' }));

	it('a button', () => expectAccessible(Button, { children: text('Continue') }));

	it('the account at a glance, with nothing to say yet', () =>
		expectAccessible(AccountSummary, { twoStep: null, sessions: null, applications: null }));

	it('the account at a glance', () =>
		expectAccessible(AccountSummary, {
			twoStep: {
				enabled: true,
				offered: true,
				asks_password: true,
				authenticator: null,
				passkeys: [],
				recovery_codes_left: 10
			},
			sessions: [
				{ id: '1', current: true, ip: '::1', user_agent: '', signed_in_at: '', expires_at: '' },
				{ id: '2', current: false, ip: '::1', user_agent: '', signed_in_at: '', expires_at: '' }
			],
			applications: []
		}));

	it('who the account is', () =>
		expectAccessible(ProfileIntro, {
			user: {
				id: '1',
				email: 'ada@example.com',
				first_name: 'Ada',
				last_name: 'Lovelace',
				email_verified: false,
				created_at: '',
				last_login_at: null
			} as never,
			twoStep: null
		}));

	it('a button that is waiting', () =>
		expectAccessible(Button, { loading: true, children: text('Saving') }));

	it('a button in every variant', async () => {
		for (const variant of ['solid', 'subtle', 'outline', 'ghost', 'plain', 'link'] as const)
			await expectAccessible(Button, {
				variant,
				colorPalette: 'danger' as const,
				children: text('Remove')
			});
	});

	it('a link shaped like a button', () =>
		expectAccessible(LinkButton, { href: '/next', block: true, children: text('Continue') }));

	it('a disabled link shaped like a button', () =>
		expectAccessible(LinkButton, { href: '/next', disabled: true, children: text('Continue') }));

	it('a panel with a description, a status and actions', () =>
		expectAccessible(Panel, {
			title: 'Password',
			description: 'Changing it signs you out elsewhere',
			icon: 'ri-lock-line',
			meta: text('On'),
			footer: text('Save'),
			children: text('Fields')
		}));

	it('a panel whose list meets its edges', () =>
		expectAccessible(Panel, { title: 'Sessions', flush: true, children: text('Rows') }));

	it('a checkbox with a remark under it', () =>
		expectAccessible(Checkbox, { checked: true, children: text('Stay signed in') }));

	it('a disabled checkbox', () =>
		expectAccessible(Checkbox, { checked: false, disabled: true, children: text('Remember') }));

	it('a field nobody may change', () =>
		expectAccessible(TextField, { label: 'Email', value: 'ada@example.com', readonly: true }));

	it('a password field that was refused, with a way out', () =>
		expectAccessible(PasswordField, {
			label: 'Password',
			value: 'x',
			error: 'That is not right',
			aside: { label: 'Forgot password?', href: '/forgot-password' }
		}));

	it('a spinner that says what it waits for', () =>
		expectAccessible(Spinner, { label: 'Loading', size: 'lg' }));

	it('the shape of what is loading', () => expectAccessible(Skeleton, { lines: 3 }));

	it('a badge with an icon', () =>
		expectAccessible(Badge, {
			tone: 'success',
			icon: 'ri-check-line',
			children: text('Verified')
		}));

	it('an alert in every tone', async () => {
		for (const tone of ['danger', 'warning', 'info', 'success'] as const)
			await expectAccessible(Alert, { tone, children: text('Something happened') });
	});

	it('a note', () =>
		expectAccessible(Note, { boxed: true, align: 'center', children: text('Hint') }));

	it('an alert', () => expectAccessible(Alert, { tone: 'warning', children: text('Careful') }));

	it('a yes or no answer', () =>
		expectAccessible(Decision, { yes: 'Allow', no: 'Deny', onanswer: nothing }));

	it('the terms to accept', () =>
		expectAccessible(LegalLinkList, {
			links: [
				{ label: 'Terms of service', href: 'https://example.com/terms' },
				{ label: 'Privacy policy', href: 'https://example.com/privacy' }
			]
		}));

	it('recovery codes shown once', () =>
		expectAccessible(RecoveryCodeSheet, {
			codes: ['abcde-fghjk', 'mnpqr-stuvw'],
			ondone: nothing
		}));

	it('an authenticator being set up', () =>
		expectAccessible(AuthenticatorSetup, {
			begin: () => new Promise(nothing),
			confirm: () => new Promise(nothing),
			onadded: nothing,
			oncancel: nothing
		}));

	it('a passkey being added', () =>
		expectAccessible(PasskeyAdd, {
			begin: () => new Promise(nothing),
			finish: () => new Promise(nothing),
			onadded: nothing,
			oncancel: nothing
		}));

	it('a password asked before a change', () =>
		expectAccessible(PresenceCheck, {
			asksPassword: true,
			confirm: 'Remove',
			onconfirm: nothing,
			oncancel: nothing
		}));

	it('the list of devices signed in', () =>
		expectAccessible(SessionList, {
			timezone: 'UTC',
			sessions: [
				{
					id: '1',
					ip: '203.0.113.7',
					user_agent: 'Mozilla/5.0 (Macintosh) Chrome/140.0',
					signed_in_at: '2026-10-01T10:00:00Z',
					expires_at: '2026-11-01T10:00:00Z',
					current: true
				},
				{
					id: '2',
					ip: '',
					user_agent: '',
					signed_in_at: '2026-10-02T10:00:00Z',
					expires_at: '2026-11-02T10:00:00Z',
					current: false
				}
			]
		}));

	it('a device code, in two groups of four', () =>
		expectAccessible(DeviceCodeField, { label: 'Code', value: 'BCDF-GH', hint: 'Like BCDF-GHJK' }));

	it('where a device asked from', () =>
		expectAccessible(DeviceOrigin, {
			origin: {
				ip: '203.0.113.7',
				user_agent: 'Mozilla/5.0 (Macintosh) Chrome/140.0',
				at: '2026-11-22T10:00:00Z'
			}
		}));

	it("a new password's rules, some met and some not", () =>
		expectAccessible(PasswordRules, { id: 'rules', password: 'short', confirm: 'short' }));

	it('a new password, typed twice, with its rules', () =>
		expectAccessible(NewPasswordFields, {
			password: 'a-long-enough-one',
			confirm: 'a-long-enough-on'
		}));
});
