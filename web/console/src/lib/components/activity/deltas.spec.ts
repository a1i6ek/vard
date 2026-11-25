import { describe, expect, it } from 'vitest';
import { countDelta, rateDelta, successRate } from './deltas';

describe('how the dashboard says a number moved', () => {
	it('nothing then nothing says nothing', () => {
		expect(countDelta(0, 0, 'up', 14)).toBeNull();
	});

	it('something after nothing is a count, not an infinite share', () => {
		expect(countDelta(4, 0, 'up', 14)).toMatchObject({ text: '+4', direction: 'up', good: true });
	});

	it('more refused sign-ins is bad news', () => {
		expect(countDelta(15, 10, 'down', 7)).toMatchObject({
			text: '50%',
			direction: 'up',
			good: false
		});
	});

	it('fewer refused sign-ins is good news', () => {
		expect(countDelta(5, 10, 'down', 7)).toMatchObject({
			text: '50%',
			direction: 'down',
			good: true
		});
	});

	it('no change is neither', () => {
		expect(countDelta(7, 7, 'up', 7)).toMatchObject({ direction: 'flat', good: null });
	});

	it('the success rate leaves out a period with no attempts', () => {
		expect(successRate({ succeeded: 0, failed: 0, blocked: 0 })).toBeNull();
		expect(successRate({ succeeded: 3, failed: 1, blocked: 0 })).toBe(75);
	});

	it('the rate moves in points', () => {
		expect(rateDelta(90, 80, 14)).toMatchObject({ text: '10 pts', direction: 'up', good: true });
		expect(rateDelta(90, null, 14)).toBeNull();
	});
});
