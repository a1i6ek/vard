import type { LogFilter } from '#lib/api/index.js';
import { actionsIn, categoryLabels, refusedActions, type Category } from './actions';

export type Kind = Category | 'refused';

export const kindLabels: Record<Kind, string> = {
	...categoryLabels,
	refused: 'Refused sign-ins'
};

export type ActorType = 'admin' | 'user' | 'system';

export const actorTypeLabels: Record<ActorType, string> = {
	admin: 'Admins',
	user: 'People',
	system: 'The server'
};

/** How far back the log is read: a moment ago up to now, or days chosen. */
export type Range = '1h' | '24h' | '7d' | '30d' | 'custom';

export const rangeLabels: Record<Range, string> = {
	'1h': 'Last hour',
	'24h': 'Last 24 hours',
	'7d': 'Last 7 days',
	'30d': 'Last 30 days',
	custom: 'Custom'
};

const rangeHours: Record<Exclude<Range, 'custom'>, number> = {
	'1h': 1,
	'24h': 24,
	'7d': 24 * 7,
	'30d': 24 * 30
};

/** Everything the logs page is narrowed by, as its address says it. The
    address holds the range's name rather than a time, so a link to "the last
    hour" stays the last hour. */
export type LogView = {
	q: string;
	kinds: Kind[];
	actor: string;
	actorType: ActorType | '';
	ip: string;
	range: Range | '';
	from: string;
	to: string;
	filter: LogFilter;
};

const isKind = (value: string): value is Kind => value in kindLabels;
const isActorType = (value: string): value is ActorType => value in actorTypeLabels;
const isRange = (value: string): value is Range => value in rangeLabels;
const day = (value: string | null) => (value && /^\d{4}-\d{2}-\d{2}$/.test(value) ? value : '');

/** The actions a set of kinds covers; none when they cover every action
    there is, which is the same as not asking. */
export function actionsOf(kinds: Kind[]): string[] {
	const every = (Object.keys(categoryLabels) as Category[]).every((kind) => kinds.includes(kind));
	if (kinds.length === 0 || every) return [];

	const actions = new Set<string>();
	for (const kind of kinds) {
		for (const action of kind === 'refused' ? refusedActions : actionsIn(kind)) actions.add(action);
	}
	return [...actions];
}

export function filterFrom(params: URLSearchParams, now = new Date()): LogView {
	const q = params.get('q')?.trim() ?? '';
	const kinds = [...new Set(params.getAll('kind').filter(isKind))];
	const actor = params.get('actor')?.trim() ?? '';
	const rawActorType = params.get('actor_type') ?? '';
	const actorType = isActorType(rawActorType) ? rawActorType : '';
	const ip = params.get('ip')?.trim().slice(0, 45) ?? '';
	const from = day(params.get('from'));
	const to = day(params.get('to'));
	const rawRange = params.get('range') ?? '';
	// Days in the address without a range are a custom range: the links the
	// overview makes name days alone.
	const range: Range | '' = isRange(rawRange) ? rawRange : from || to ? 'custom' : '';

	const actions = actionsOf(kinds);

	let since = '';
	let until = '';
	if (range === 'custom') {
		since = from;
		until = to;
	} else if (range) {
		since = new Date(now.getTime() - rangeHours[range] * 3_600_000).toISOString();
	}

	return {
		q,
		kinds,
		actor,
		actorType,
		ip,
		range,
		from: range === 'custom' ? from : '',
		to: range === 'custom' ? to : '',
		filter: {
			q: q || undefined,
			actions: actions.length > 0 ? actions : undefined,
			actor: actor || undefined,
			actor_types: actorType ? [actorType] : undefined,
			ip: ip || undefined,
			from: since || undefined,
			to: until || undefined
		}
	};
}

/** True when anything narrows the log. */
export function isFiltered(view: LogView): boolean {
	// A custom range with no day chosen yet narrows nothing.
	const ranged = view.range !== '' && (view.range !== 'custom' || Boolean(view.from || view.to));
	return Boolean(view.q || view.kinds.length || view.actor || view.actorType || view.ip || ranged);
}

type HrefView = Partial<Omit<LogView, 'filter' | 'kinds'>> & { kind?: Kind | Kind[] };

export function logsHref(base: string, view: HrefView): string {
	const params = new URLSearchParams();
	for (const [key, value] of Object.entries(view)) {
		if (key === 'kind') {
			for (const kind of ([] as Kind[]).concat(value as Kind | Kind[])) params.append('kind', kind);
		} else if (value) {
			params.set(key === 'actorType' ? 'actor_type' : key, value as string);
		}
	}

	const query = params.toString();
	return query ? `${base}?${query}` : base;
}
