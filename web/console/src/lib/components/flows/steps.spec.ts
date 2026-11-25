import { describe, expect, it } from 'vitest';
import { NEW_FLOW, freeSlug, methodsOf, problemsOf, slugFrom, withStep } from './steps';

describe('a flow edited with switches', () => {
	it('keeps its steps in one order, whatever order they are turned on in', () => {
		const steps = withStep(withStep(['identifier', 'social'], 'consent', true), 'password', true);
		expect(steps).toEqual(['identifier', 'password', 'social', 'consent']);
	});

	it('always asks who is signing in', () => {
		expect(withStep(['password'], 'identifier', false)).toEqual(['identifier', 'password']);
	});

	it('turns a step off', () => {
		expect(withStep(NEW_FLOW.steps, 'social', false)).toEqual(['identifier', 'password']);
	});

	it('a new flow saves once it is named', () => {
		expect(problemsOf(NEW_FLOW)).toEqual(['Name the flow.', expect.any(String)]);
		expect(problemsOf({ ...NEW_FLOW, name: 'Staff', slug: 'staff' })).toEqual([]);
	});

	it('nobody could sign in without a password or another account', () => {
		const draft = {
			...NEW_FLOW,
			name: 'x',
			slug: 'x',
			steps: withStep(['identifier'], 'email_code', true)
		};
		expect(problemsOf(draft)).toContain(
			'Turn on passwords or other accounts: without one, nobody can sign in.'
		);
	});

	it('two-step is required only once it is offered', () => {
		const draft = { ...NEW_FLOW, name: 'x', slug: 'x', require_second_factor: true };
		expect(problemsOf(draft)).toContain('Turn on two-step verification to require it.');
		expect(problemsOf({ ...draft, steps: withStep(draft.steps, 'totp', true) })).toEqual([]);
	});

	it('says how people sign in, in a few words', () => {
		expect(methodsOf(['identifier', 'password', 'social', 'totp'])).toEqual([
			'Password',
			'Other accounts',
			'Two-step'
		]);
	});
});

describe('an identifier made from a name', () => {
	it('is lower case words joined by dashes', () => {
		expect(slugFrom('Staff Sign-in!')).toBe('staff-sign-in');
	});

	it('is numbered when taken', () => {
		expect(freeSlug('staff', ['staff', 'staff-2'])).toBe('staff-3');
	});
});
