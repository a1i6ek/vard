/** How a figure moved since the period before, and whether that is good
    news: fewer refused sign-ins is down and good. */
export type Delta = {
	/** The change itself: "12%", "3 pts", "+4". */
	text: string;
	direction: 'up' | 'down' | 'flat';
	/** Null when the change is neither good nor bad. */
	good: boolean | null;
	/** Said in full: "up 12% on the 14 days before". */
	title: string;
};
