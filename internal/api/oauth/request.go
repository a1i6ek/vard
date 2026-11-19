package oauth

import (
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/a1i6ek/vard/internal/authreq"
	"github.com/a1i6ek/vard/internal/model"
	"github.com/a1i6ek/vard/internal/oauth"
	"github.com/a1i6ek/vard/internal/oidc"
)

func authorizeParams(c *gin.Context) (oauth.AuthorizeParams, error) {
	if err := c.Request.ParseForm(); err != nil {
		return oauth.AuthorizeParams{}, err
	}

	params := requestParams(c.Request.Form)

	// A parameter given twice, in the address or the body or once in each,
	// is ambiguous — which redirect_uri, which state? — and refused (RFC 6749
	// section 3.1). The provider answers it with its error page, since no
	// redirect URI can be trusted yet. resource may be repeated (RFC 8707
	// §2), and more than one is refused as invalid_target.
	for key, values := range c.Request.Form {
		if len(values) > 1 && key != "resource" {
			params.Repeated = key
			break
		}
	}

	return params, nil
}

// requestParams is an authorization request as either endpoint reads it.
func requestParams(form url.Values) oauth.AuthorizeParams {
	return oauth.AuthorizeParams{
		ClientID:            form.Get("client_id"),
		RedirectURI:         form.Get("redirect_uri"),
		ResponseType:        form.Get("response_type"),
		ResponseMode:        form.Get("response_mode"),
		Scope:               form.Get("scope"),
		State:               form.Get("state"),
		Nonce:               form.Get("nonce"),
		CodeChallenge:       form.Get("code_challenge"),
		CodeChallengeMethod: form.Get("code_challenge_method"),
		Resource:            form.Get("resource"),
		ExtraResource:       len(form["resource"]) > 1,
		Prompt:              form.Get("prompt"),
		MaxAge:              form.Get("max_age"),
		LoginHint:           form.Get("login_hint"),
		RequestURI:          form.Get("request_uri"),
		DPoPKey:             form.Get("dpop_jkt"),
		RequestObject:       form.Get("request"),
		Registration:        form.Get("registration"),
	}
}

// pushedParams is a pushed authorization request: the client's credentials,
// the request as the authorization endpoint would read it, and its DPoP
// proof, "" when it sent none.
func pushedParams(c *gin.Context) (oauth.ClientAuth, oauth.AuthorizeParams, string, error) {
	form, err := readForm(c)
	if err != nil {
		return oauth.ClientAuth{}, oauth.AuthorizeParams{}, "", err
	}

	auth, err := clientAuth(c, form)
	if err != nil {
		return oauth.ClientAuth{}, oauth.AuthorizeParams{}, "", err
	}

	proof, err := dpopProof(c)
	if err != nil {
		return oauth.ClientAuth{}, oauth.AuthorizeParams{}, "", err
	}

	return auth, requestParams(form), proof, nil
}

func logoutParams(c *gin.Context) (oidc.LogoutParams, error) {
	if err := c.Request.ParseForm(); err != nil {
		return oidc.LogoutParams{}, err
	}

	form := c.Request.Form
	return oidc.LogoutParams{
		IDTokenHint:           form.Get("id_token_hint"),
		ClientID:              form.Get("client_id"),
		PostLogoutRedirectURI: form.Get("post_logout_redirect_uri"),
		State:                 form.Get("state"),
	}, nil
}

func tokenParams(c *gin.Context) (oauth.TokenParams, error) {
	form, err := readForm(c)
	if err != nil {
		return oauth.TokenParams{}, err
	}

	auth, err := clientAuth(c, form)
	if err != nil {
		return oauth.TokenParams{}, err
	}

	proof, err := dpopProof(c)
	if err != nil {
		return oauth.TokenParams{}, err
	}

	if len(form["resource"]) > 1 {
		return oauth.TokenParams{}, oauth.ErrSeveralResources
	}

	return oauth.TokenParams{
		Client:       auth,
		GrantType:    form.Get("grant_type"),
		Code:         form.Get("code"),
		RedirectURI:  form.Get("redirect_uri"),
		CodeVerifier: form.Get("code_verifier"),
		RefreshToken: form.Get("refresh_token"),
		DeviceCode:   form.Get("device_code"),
		Scope:        form.Get("scope"),
		Resource:     form.Get("resource"),
		DPoP:         proof,
	}, nil
}

func tokenOnlyParams(c *gin.Context) (oauth.ClientAuth, string, error) {
	form, err := readForm(c)
	if err != nil {
		return oauth.ClientAuth{}, "", err
	}

	auth, err := clientAuth(c, form)
	if err != nil {
		return oauth.ClientAuth{}, "", err
	}

	return auth, form.Get("token"), nil
}

func readForm(c *gin.Context) (url.Values, error) {
	if c.Request.Method != http.MethodPost {
		return nil, invalidRequest("this endpoint only accepts POST")
	}

	media, _, _ := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if media != "application/x-www-form-urlencoded" {
		return nil, invalidRequest("the body must be application/x-www-form-urlencoded")
	}

	if err := c.Request.ParseForm(); err != nil {
		return nil, invalidRequest("the body could not be read")
	}

	// Parameters that mean something must not be repeated (RFC 6749 section
	// 3.2). resource may be (RFC 8707 §2), and more than one is refused by
	// what reads it, as invalid_target.
	for key, values := range c.Request.PostForm {
		if len(values) > 1 && key != "resource" {
			return nil, invalidRequest(key + " is repeated")
		}
	}

	return c.Request.PostForm, nil
}

// clientAuth reads how the client identified itself.
func clientAuth(c *gin.Context, form url.Values) (oauth.ClientAuth, error) {
	auth, err := credentials(c, form)
	auth.Endpoint = c.Request.URL.Path
	return auth, err
}

// credentials reads how the client authenticates: a signed assertion
// (RFC 7523 §2.2), or its id and secret. Basic credentials are form-decoded
// (RFC 6749 2.3.1). A client authenticates one way only (RFC 6749 §2.3).
func credentials(c *gin.Context, form url.Values) (oauth.ClientAuth, error) {
	if form.Has("client_assertion_type") || form.Has("client_assertion") {
		_, _, basic := c.Request.BasicAuth()
		switch {
		case basic || form.Get("client_secret") != "":
			return oauth.ClientAuth{}, invalidRequest("authenticate one way: a client assertion, or a client secret, not both")
		case form.Get("client_assertion_type") != oauth.ClientAssertionType:
			return oauth.ClientAuth{}, invalidRequest("client_assertion_type must be " + oauth.ClientAssertionType)
		case form.Get("client_assertion") == "":
			return oauth.ClientAuth{}, invalidRequest("client_assertion is required with client_assertion_type")
		}
		return oauth.ClientAuth{ID: form.Get("client_id"), Assertion: form.Get("client_assertion"), Method: model.AuthPrivateKeyJWT}, nil
	}

	if id, secret, ok := c.Request.BasicAuth(); ok {
		if form.Get("client_secret") != "" {
			return oauth.ClientAuth{}, invalidRequest("send the client secret one way, not in both the header and the body")
		}

		decodedID, err1 := url.QueryUnescape(id)
		decodedSecret, err2 := url.QueryUnescape(secret)
		if err1 != nil || err2 != nil {
			return oauth.ClientAuth{}, &authreq.Error{Code: authreq.ErrInvalidClient, Description: "the Authorization header could not be read", Status: http.StatusUnauthorized}
		}

		if bodyID := form.Get("client_id"); bodyID != "" && bodyID != decodedID {
			return oauth.ClientAuth{}, invalidRequest("client_id in the body does not match the Authorization header")
		}

		return oauth.ClientAuth{ID: decodedID, Secret: decodedSecret, Method: model.AuthClientSecretBasic}, nil
	}

	if secret := form.Get("client_secret"); secret != "" {
		return oauth.ClientAuth{ID: form.Get("client_id"), Secret: secret, Method: model.AuthClientSecretPost}, nil
	}

	return oauth.ClientAuth{ID: form.Get("client_id"), Method: model.AuthNone}, nil
}

// bearer reads the access token from the Authorization header, or from the
// form of a POST (RFC 6750 section 2).
func bearer(c *gin.Context) (string, error) {
	if header := c.GetHeader("Authorization"); header != "" {
		scheme, token, found := strings.Cut(header, " ")
		if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			return "", &authreq.Error{Code: authreq.ErrInvalidToken, Description: "the Authorization header must be Bearer <access token>", Status: http.StatusUnauthorized}
		}
		return strings.TrimSpace(token), nil
	}

	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err == nil {
			if token := c.Request.PostForm.Get("access_token"); token != "" {
				return token, nil
			}
		}
	}

	return "", &authreq.Error{Code: authreq.ErrInvalidToken, Description: "an access token is required", Status: http.StatusUnauthorized}
}

// deviceParams is a device asking to sign someone in: its credentials, and
// what it asks for.
func deviceParams(c *gin.Context) (oauth.ClientAuth, string, string, error) {
	form, err := readForm(c)
	if err != nil {
		return oauth.ClientAuth{}, "", "", err
	}

	auth, err := clientAuth(c, form)
	if err != nil {
		return oauth.ClientAuth{}, "", "", err
	}

	if len(form["resource"]) > 1 {
		return oauth.ClientAuth{}, "", "", oauth.ErrSeveralResources
	}

	return auth, form.Get("scope"), form.Get("resource"), nil
}

// dpopProof is the request's DPoP header (RFC 9449 §4.1): one, or none.
func dpopProof(c *gin.Context) (string, error) {
	proofs := c.Request.Header.Values("DPoP")
	if len(proofs) > 1 {
		return "", &authreq.Error{Code: oauth.ErrInvalidDPoPProof, Description: "send one DPoP proof", Status: http.StatusBadRequest}
	}
	if len(proofs) == 0 {
		return "", nil
	}
	return proofs[0], nil
}

// presentation reads a protected request's token, as Bearer or as DPoP with
// its proof, for the address the client sent it to.
func presentation(c *gin.Context, target string) (string, oauth.Presentation, error) {
	how := oauth.Presentation{Method: c.Request.Method, URL: target, Scheme: "Bearer"}

	if header := c.GetHeader("Authorization"); header != "" {
		scheme, token, found := strings.Cut(header, " ")
		if found && strings.EqualFold(scheme, oauth.TokenTypeDPoP) && strings.TrimSpace(token) != "" {
			proof, err := dpopProof(c)
			if err != nil {
				return "", how, err
			}
			how.Scheme, how.Proof = oauth.TokenTypeDPoP, proof
			return strings.TrimSpace(token), how, nil
		}
	}

	token, err := bearer(c)
	return token, how, err
}
