import { describe, expect, it } from 'vitest';
import { actionsIn, categoryLabels, refusedActions, type Category } from './actions';
import { actionsOf, filterFrom, isFiltered, logsHref } from './filters';

const params = (query: string) => new URLSearchParams(query);
const now = new Date('2026-11-26T12:00:00Z');

describe('the logs page reads its filters from its address', () => {
	it('nothing in the address narrows nothing', () => {
		const view = filterFrom(params(''), now);
		expect(isFiltered(view)).toBe(false);
		expect(view.filter).toEqual({});
	});

	it('several kinds ask for the actions of each', () => {
		const view = filterFrom(params('kind=refused&kind=apis&kind=refused'), now);
		expect(view.kinds).toEqual(['refused', 'apis']);
		expect(new Set(view.filter.actions)).toEqual(
			new Set([...refusedActions, ...actionsIn('apis')])
		);
	});

	it('every kind there is is the same as not asking', () => {
		expect(actionsOf(Object.keys(categoryLabels) as Category[])).toEqual([]);
	});

	it('a kind it does not know is left out', () => {
		expect(filterFrom(params('kind=everything'), now).kinds).toEqual([]);
	});

	it('a preset reads back from now', () => {
		const view = filterFrom(params('range=24h'), now);
		expect(view.filter.from).toBe('2026-11-25T12:00:00.000Z');
		expect(view.filter.to).toBeUndefined();
	});

	it('days without a range are a custom range, as the overview links them', () => {
		const view = filterFrom(params('from=2026-11-01&to=2026-11-02'), now);
		expect(view.range).toBe('custom');
		expect(view.filter).toMatchObject({ from: '2026-11-01', to: '2026-11-02' });
	});

	it('days beside a preset are ignored', () => {
		const view = filterFrom(params('range=1h&from=2026-11-01'), now);
		expect(view.from).toBe('');
	});

	it('who acted and from where', () => {
		const view = filterFrom(params('actor_type=system&ip=10.0.0.1&actor_type=robot'), now);
		expect(view.filter).toMatchObject({ actor_types: ['system'], ip: '10.0.0.1' });
	});

	it('an actor type it does not know is no filter', () => {
		expect(filterFrom(params('actor_type=robot'), now).filter.actor_types).toBeUndefined();
	});
});

describe('links into the log', () => {
	it('repeat each kind and spell the actor type as the address does', () => {
		expect(logsHref('/logs', { kind: ['refused', 'access'], actorType: 'admin' })).toBe(
			'/logs?kind=refused&kind=access&actor_type=admin'
		);
	});

	it('leave out what is empty', () => {
		expect(logsHref('/logs', { actor: '', from: '2026-11-01' })).toBe('/logs?from=2026-11-01');
	});
});
