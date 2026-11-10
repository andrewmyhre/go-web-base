package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

// Google's OpenID Connect endpoints. They are fixed rather than discovered
// so the server starts without reaching Google; the signing keys are
// fetched on first use and cached.
const (
	GoogleIssuer   = "https://accounts.google.com"
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleJWKSURL  = "https://www.googleapis.com/oauth2/v3/certs"
)

// GoogleConfig configures Google sign-in. The endpoint fields default to
// Google's and exist so tests can point them at a fake.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is the absolute callback URL registered with the Google
	// OAuth client. It is configured, never derived from the request's Host
	// header, which the client controls.
	RedirectURL string

	Issuer, AuthURL, TokenURL, JWKSURL string
	// HTTPClient makes the token and key requests; nil for one with a 10
	// second timeout.
	HTTPClient *http.Client
}

// Identity is what a verified ID token says about the user. Sub, the
// Google account ID, is the identity. Email is set only when Google says
// it is verified, and it is for display: nothing matches on it.
type Identity struct {
	Sub     string
	Name    string
	Email   string
	Picture string
}

// Google signs users in with Google's OpenID Connect authorization-code
// flow, with PKCE (S256) and a nonce.
type Google struct {
	oauth    oauth2.Config
	verifier *oidc.IDTokenVerifier
	client   *http.Client
}

// NewGoogle builds the Google sign-in client.
func NewGoogle(c GoogleConfig) (*Google, error) {
	if c.ClientID == "" || c.ClientSecret == "" {
		return nil, errors.New("google sign-in needs a client id and a client secret")
	}
	u, err := url.Parse(c.RedirectURL)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return nil, fmt.Errorf("google redirect url %q is not an absolute url", c.RedirectURL)
	}
	if u.Scheme != "https" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
		return nil, fmt.Errorf("google redirect url %q must be https", c.RedirectURL)
	}
	if c.Issuer == "" {
		c.Issuer = GoogleIssuer
	}
	if c.AuthURL == "" {
		c.AuthURL = googleAuthURL
	}
	if c.TokenURL == "" {
		c.TokenURL = googleTokenURL
	}
	if c.JWKSURL == "" {
		c.JWKSURL = googleJWKSURL
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	keys := oidc.NewRemoteKeySet(oidc.ClientContext(context.Background(), c.HTTPClient), c.JWKSURL)
	return &Google{
		oauth: oauth2.Config{
			ClientID:     c.ClientID,
			ClientSecret: c.ClientSecret,
			RedirectURL:  c.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:   c.AuthURL,
				TokenURL:  c.TokenURL,
				AuthStyle: oauth2.AuthStyleInHeader,
			},
			Scopes: []string{oidc.ScopeOpenID, "email", "profile"},
		},
		// Verify checks the signature against the JWKS (RS256 only), iss,
		// that aud contains the client id, and exp.
		verifier: oidc.NewVerifier(c.Issuer, keys, &oidc.Config{
			ClientID:             c.ClientID,
			SupportedSigningAlgs: []string{oidc.RS256},
		}),
		client: c.HTTPClient,
	}, nil
}

// AuthCodeURL is the Google consent URL for one sign-in attempt.
func (g *Google) AuthCodeURL(state, nonce, verifier string) string {
	return g.oauth.AuthCodeURL(state,
		oauth2.S256ChallengeOption(verifier),
		oidc.Nonce(nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
}

// Exchange trades code for tokens, presenting the PKCE verifier, and
// returns the identity in the verified ID token. The token must carry
// nonce, and an azp claim, when present, must be our client id.
func (g *Google) Exchange(ctx context.Context, code, verifier, nonce string) (Identity, error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, g.client)
	tok, err := g.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return Identity{}, fmt.Errorf("exchange code: %w", err)
	}
	raw, _ := tok.Extra("id_token").(string)
	if raw == "" {
		return Identity{}, errors.New("token response has no id_token")
	}
	idt, err := g.verifier.Verify(ctx, raw)
	if err != nil {
		return Identity{}, fmt.Errorf("verify id token: %w", err)
	}
	if subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(nonce)) != 1 {
		return Identity{}, errors.New("id token nonce does not match the sign-in attempt")
	}
	var claims struct {
		Sub           string `json:"sub"`
		Azp           string `json:"azp"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
	}
	if err := idt.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode id token claims: %w", err)
	}
	if claims.Azp != "" && claims.Azp != g.oauth.ClientID {
		return Identity{}, fmt.Errorf("id token azp %q is not our client id", claims.Azp)
	}
	if claims.Sub == "" {
		return Identity{}, errors.New("id token has no sub")
	}
	id := Identity{Sub: claims.Sub, Name: claims.Name, Picture: claims.Picture}
	if claims.EmailVerified {
		id.Email = claims.Email
	}
	return id, nil
}
