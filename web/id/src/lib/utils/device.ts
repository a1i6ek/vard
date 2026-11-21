/** A device code as it is typed: upper case, its letters and digits only, and
 *  the dash put back between the two groups of four, as the device shows it.
 *  The server compares it without the dash, in any case. */
export function formatUserCode(typed: string): string {
	const letters = typed
		.toUpperCase()
		.replace(/[^A-Z0-9]/g, '')
		.slice(0, 8);
	return letters.length > 4 ? `${letters.slice(0, 4)}-${letters.slice(4)}` : letters;
}
