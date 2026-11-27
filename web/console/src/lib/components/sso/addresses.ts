/** The addresses a connection's provider is given, built as the server builds them
    (model.SSOConnection.base): under <issuer>/oauth2/sso/<identifier>. */
export function providerAddresses(issuer: string, identifier: string) {
	const base = `${issuer.replace(/\/+$/, '')}/oauth2/sso/${identifier}`;
	return {
		callback: `${base}/callback`,
		acs: `${base}/acs`,
		metadata: `${base}/metadata`,
		entityId: `${base}/metadata`
	};
}
