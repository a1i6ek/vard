import type { Delta } from '#lib/components/ui/index.js';

/**
 * How a count moved from the period before to this one, as a share of what
 * it was. `better` says which way is good news; nothing to compare (both
 * nought) says nothing.
 */
export function countDelta(
	now: number,
	before: number,
	better: 'up' | 'down',
	days: number
): Delta | null {
	if (now === 0 && before === 0) return null;

	const period = `the ${days} days before`;

	if (before === 0) {
		return {
			text: `+${now.toLocaleString('en')}`,
			direction: 'up',
			good: better === 'up',
			title: `${now.toLocaleString('en')} more than none in ${period}`
		};
	}

	const change = Math.round(((now - before) / before) * 100);
	if (change === 0)
		return { text: '0%', direction: 'flat', good: null, title: `the same as ${period}` };

	const direction = change > 0 ? 'up' : 'down';
	return {
		text: `${Math.abs(change)}%`,
		direction,
		good: direction === better,
		title: `${direction} ${Math.abs(change)}% on ${period}`
	};
}

/** The share of sign-ins that succeeded, as a whole percentage; none without attempts. */
export function successRate(signIns: {
	succeeded: number;
	failed: number;
	blocked: number;
}): number | null {
	const attempts = signIns.succeeded + signIns.failed + signIns.blocked;
	return attempts === 0 ? null : Math.round((signIns.succeeded / attempts) * 100);
}

/** How the success rate moved, in percentage points; a higher rate is good. */
export function rateDelta(now: number | null, before: number | null, days: number): Delta | null {
	if (now === null || before === null) return null;

	const points = now - before;
	const period = `the ${days} days before`;
	if (points === 0)
		return { text: '0 pts', direction: 'flat', good: null, title: `the same as ${period}` };

	const direction = points > 0 ? 'up' : 'down';
	return {
		text: `${Math.abs(points)} pts`,
		direction,
		good: direction === 'up',
		title: `${direction} ${Math.abs(points)} percentage points on ${period}`
	};
}
