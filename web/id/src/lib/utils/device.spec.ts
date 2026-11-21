import { describe, expect, it } from 'vitest';
import { formatUserCode } from './device';

describe('a device code as it is typed', () => {
	it.each([
		['bcdf', 'BCDF'],
		['bcdfg', 'BCDF-G'],
		['bcdfghjk', 'BCDF-GHJK'],
		['BCDF-GHJK', 'BCDF-GHJK'],
		[' bcdf ghjk ', 'BCDF-GHJK'],
		['bc-df-gh-jk', 'BCDF-GHJK'],
		['bcdfghjkxyz', 'BCDF-GHJK'],
		['', '']
	])('%j reads as %j', (typed, shown) => expect(formatUserCode(typed)).toBe(shown));
});
