package oidc

import (
	"context"
	"fmt"
	"net/url"

	coreoidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type Identity struct {
	Username string
}

type Provider struct {
	oauth2        oauth2.Config
	verifier      *coreoidc.IDTokenVerifier
	clientID      string
	endSessionURL string
}

// New fetches the discovery document. A failure here is fatal at startup rather
// than deferred, so a misconfigured issuer never turns into a login-time 500.
func New(ctx context.Context, cfg Config) (*Provider, error) {
	provider, err := coreoidc.NewProvider(ctx, cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery for %q: %w", cfg.Issuer, err)
	}

	var metadata struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&metadata); err != nil {
		return nil, fmt.Errorf("reading the discovery document: %w", err)
	}

	return &Provider{
		oauth2: oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{coreoidc.ScopeOpenID, coreoidc.ScopeProfile},
		},
		verifier:      provider.Verifier(&coreoidc.Config{ClientID: cfg.ClientID}),
		clientID:      cfg.ClientID,
		endSessionURL: metadata.EndSessionEndpoint,
	}, nil
}

func (p *Provider) AuthCodeURL(state, nonce string) string {
	return p.oauth2.AuthCodeURL(state, coreoidc.Nonce(nonce))
}

func (p *Provider) Exchange(ctx context.Context, code, nonce string) (Identity, error) {
	token, err := p.oauth2.Exchange(ctx, code)
	if err != nil {
		return Identity{}, fmt.Errorf("exchanging the authorization code: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return Identity{}, fmt.Errorf("the token response carries no id_token")
	}

	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verifying the id_token: %w", err)
	}

	// go-oidc parses the nonce but leaves the comparison to the caller.
	if idToken.Nonce != nonce {
		return Identity{}, fmt.Errorf("nonce mismatch")
	}

	var claims struct {
		PreferredUsername string `json:"preferred_username"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("reading the id_token claims: %w", err)
	}

	username, err := AreaName(claims.PreferredUsername)
	if err != nil {
		return Identity{}, err
	}

	return Identity{Username: username}, nil
}

// LogoutURL ends the session at the provider as well. Without it the next click
// on "Sign in" would silently log the same user back in.
func (p *Provider) LogoutURL(postLogoutRedirect string) string {
	if p.endSessionURL == "" {
		return postLogoutRedirect
	}

	u, err := url.Parse(p.endSessionURL)
	if err != nil {
		return postLogoutRedirect
	}

	q := u.Query()
	q.Set("client_id", p.clientID)
	q.Set("post_logout_redirect_uri", postLogoutRedirect)
	u.RawQuery = q.Encode()

	return u.String()
}
