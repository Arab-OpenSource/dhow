package oidcauth_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
	oidcmock "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc/Mock"
)

func TestReplaceKeepsUnchangedProvidersAndRebuildsChangedOnes(t *testing.T) {
	idp := oidcmock.New(t, false, nil)
	corp := config.OIDCProvider{
		ID: "corp", Kind: config.OIDCKindOIDC, Name: "Corp", Issuer: idp.Issuer(), ClientID: oidcmock.ClientID, ClientSecret: oidcmock.ClientSecret,
	}
	gh := config.OIDCProvider{ID: "gh", Kind: config.OIDCKindGitHub, Name: "GitHub", Issuer: idp.Issuer(), ClientID: "c", ClientSecret: "s"}
	r, err := oidcauth.NewRegistry([]config.OIDCProvider{corp}, oidcauth.Options{Transport: idp.Transport()})
	require.NoError(t, err)
	first, err := r.Get("corp")
	require.NoError(t, err)
	_, err = first.AuthorizeURL(context.Background(), flow())
	require.NoError(t, err)
	require.Equal(t, 1, idp.Count("discovery"))

	require.NoError(t, r.Replace([]config.OIDCProvider{corp, gh}))
	same, err := r.Get("corp")
	require.NoError(t, err)
	assert.Same(t, first, same, "an unchanged provider keeps its discovery cache")
	assert.Equal(t, []string{"corp", "gh"}, r.IDs())

	renamed := corp
	renamed.Name = "Corp login"
	require.NoError(t, r.Replace([]config.OIDCProvider{renamed}))
	next, err := r.Get("corp")
	require.NoError(t, err)
	assert.NotSame(t, first, next)
	assert.Equal(t, "Corp login", next.Info().Name)
	_, err = next.AuthorizeURL(context.Background(), flow())
	require.NoError(t, err)
	assert.Equal(t, 2, idp.Count("discovery"), "a changed provider discovers again")
	_, err = r.Get("gh")
	assert.ErrorIs(t, err, oidcauth.ErrUnknown)

	assert.Error(t, r.Replace([]config.OIDCProvider{{ID: "x", Kind: "saml"}}))
	_, err = r.Get("corp")
	assert.NoError(t, err, "a failed replace keeps the previous set")
}

func TestOutboundConnectionsRefuseLinkLocalAndMetadataAddresses(t *testing.T) {
	for addr, blocked := range map[string]bool{
		"169.254.169.254:80": true, "[fe80::1]:443": true, "100.100.100.200:80": true, "[fd00:ec2::254]:80": true,
		"127.0.0.1:8443": false, "10.0.0.5:443": false, "[2001:db8::1]:443": false,
	} {
		err := oidcauth.RefuseBlocked("tcp", addr, nil)
		assert.Equal(t, blocked, err != nil, addr)
	}
}
