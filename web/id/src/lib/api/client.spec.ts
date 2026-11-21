import { describe, expect, it } from 'vitest';

import { ApiError, request } from './client';

/** A fetch that answers every call with `status` and `body`, as the API or something in front of it would. */
function answer(status: number, body: string): typeof globalThis.fetch {
	return () =>
		Promise.resolve(
			new Response(body, { status, headers: { 'Content-Type': 'application/json' } })
		);
}

describe('a failed call', () => {
	const tests: { name: string; status: number; body: string; want: Partial<ApiError> }[] = [
		{
			name: "the API's own problem",
			status: 409,
			body: JSON.stringify({
				code: 'email_taken',
				type: 'https://id.example.com/problems/email_taken',
				title: 'Conflict',
				status: 409,
				detail: 'That email is taken.',
				params: { email: 'a@b.c' }
			}),
			want: {
				status: 409,
				code: 'email_taken',
				message: 'That email is taken.',
				params: { email: 'a@b.c' }
			}
		},
		{
			name: 'a body that is not JSON, such as a proxy error page',
			status: 502,
			body: '<html>Bad Gateway</html>',
			want: {
				status: 502,
				code: 'unknown',
				message: 'Something went wrong. Try again.',
				params: {}
			}
		},
		{
			name: 'fields of the wrong type',
			status: 500,
			body: JSON.stringify({ code: 7, detail: { text: 'no' }, params: 'nope' }),
			want: { code: 'unknown', message: 'Something went wrong. Try again.', params: {} }
		},
		{
			name: 'parameters that are neither text nor numbers, which are dropped',
			status: 400,
			body: JSON.stringify({
				code: 'invalid',
				detail: 'Bad.',
				params: { field: 'name', max: 3, nested: { a: 1 } }
			}),
			want: { code: 'invalid', params: { field: 'name', max: 3 } }
		}
	];

	for (const { name, status, body, want } of tests) {
		it(name, async () => {
			const failure = await request('/x', { fetch: answer(status, body) }).catch(
				(err: unknown) => err
			);

			expect(failure).toBeInstanceOf(ApiError);
			expect(failure).toMatchObject(want);
		});
	}
});
