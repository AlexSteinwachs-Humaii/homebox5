package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserService_RegistrationTheme(t *testing.T) {
	ctx := context.Background()
	f := newOwnershipFixture(t)
	_, invite, err := tSvc.Group.NewInvitation(f.ownerCtx, 2, time.Now().Add(time.Hour))
	require.NoError(t, err)
	for _, token := range []string{"", invite} {
		registered, err := tSvc.User.RegisterUser(ctx, UserRegistration{
			Name: fk.Str(8), Email: fk.Email(), Password: "correct-horse-battery-staple", GroupToken: token,
		})
		require.NoError(t, err)
		settings, err := tRepos.Users.GetSettings(ctx, registered.ID)
		require.NoError(t, err)
		assert.Equal(t, "cinematic", settings["theme"])
		if token != "" {
			assert.Equal(t, f.group.ID, registered.DefaultGroupID)
		}
	}
}

func TestUserService_LoginThemePreservation(t *testing.T) {
	ctx := context.Background()
	const password = "correct-horse-battery-staple"
	for _, settings := range []map[string]interface{}{nil, {}, {"theme": "light"}} {
		email := fk.Email()
		registered, err := tSvc.User.RegisterUser(ctx, UserRegistration{
			Name: fk.Str(8), Email: email, Password: password,
		})
		require.NoError(t, err)
		require.NoError(t, tRepos.Users.SetSettings(ctx, registered.ID, settings))
		_, err = tSvc.User.Login(ctx, email, password, false)
		require.NoError(t, err)
		subject := uuid.NewString()
		// First OIDC login links by email; second uses the linked identity.
		for range 2 {
			_, err = tSvc.User.LoginOIDC(ctx, "https://issuer.example", subject, email, "Existing")
			require.NoError(t, err)
			actual, err := tRepos.Users.GetSettings(ctx, registered.ID)
			require.NoError(t, err)
			assert.Equal(t, settings, actual)
		}
	}
}

func TestUserService_NewOIDCTheme(t *testing.T) {
	ctx := context.Background()
	email, subject := fk.Email(), uuid.NewString()
	_, err := tSvc.User.LoginOIDC(ctx, "https://issuer.example", subject, email, "New OIDC")
	require.NoError(t, err)
	registered, err := tRepos.Users.GetOneOIDC(ctx, "https://issuer.example", subject)
	require.NoError(t, err)
	settings, err := tRepos.Users.GetSettings(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, "cinematic", settings["theme"])
	require.NoError(t, tRepos.Users.SetSettings(ctx, registered.ID, map[string]interface{}{"theme": "light"}))
	_, err = tSvc.User.LoginOIDC(ctx, "https://issuer.example", subject, email, "New OIDC")
	require.NoError(t, err)
	settings, err = tRepos.Users.GetSettings(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, "light", settings["theme"])
}
