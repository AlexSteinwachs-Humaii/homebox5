package repo

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func userFactory() UserCreate {
	return UserCreate{
		Name:           fk.Str(10),
		Email:          fk.Email(),
		Password:       new(fk.Str(10)),
		IsSuperuser:    fk.Bool(),
		DefaultGroupID: tGroup.ID,
	}
}

func TestUserRepo_GetOneEmail(t *testing.T) {
	assert := assert.New(t)
	user := userFactory()
	ctx := context.Background()

	_, err := tRepos.Users.Create(ctx, user)
	require.NoError(t, err)

	foundUser, err := tRepos.Users.GetOneEmail(ctx, user.Email)

	assert.NotNil(foundUser)
	require.NoError(t, err)
	// Emails are canonicalized (lowercased/trimmed) on storage to keep the
	// case-sensitive DB unique constraint effective; compare against that form.
	assert.Equal(normalizeEmail(user.Email), foundUser.Email)
	assert.Equal(user.Name, foundUser.Name)

	// Cleanup
	err = tRepos.Users.DeleteAll(ctx)
	require.NoError(t, err)
}

func TestUserRepo_GetOneId(t *testing.T) {
	assert := assert.New(t)
	user := userFactory()
	ctx := context.Background()

	userOut, _ := tRepos.Users.Create(ctx, user)
	foundUser, err := tRepos.Users.GetOneID(ctx, userOut.ID)

	assert.NotNil(foundUser)
	require.NoError(t, err)
	assert.Equal(normalizeEmail(user.Email), foundUser.Email)
	assert.Equal(user.Name, foundUser.Name)

	// Cleanup
	err = tRepos.Users.DeleteAll(ctx)
	require.NoError(t, err)
}

func TestUserRepo_GetAll(t *testing.T) {
	// Setup
	toCreate := []UserCreate{
		userFactory(),
		userFactory(),
		userFactory(),
		userFactory(),
	}

	ctx := context.Background()

	created := []UserOut{}

	for _, usr := range toCreate {
		usrOut, _ := tRepos.Users.Create(ctx, usr)
		created = append(created, usrOut)
	}

	// Validate
	allUsers, err := tRepos.Users.GetAll(ctx)

	require.NoError(t, err)
	assert.Len(t, allUsers, len(created))

	for _, usr := range created {
		for _, usr2 := range allUsers {
			if usr.ID == usr2.ID {
				assert.Equal(t, usr.Email, usr2.Email)

				// Check groups are loaded
				assert.NotEqual(t, uuid.Nil, usr2.DefaultGroupID)
				assert.NotEmpty(t, usr2.GroupIDs)
			}
		}
	}

	for _, usr := range created {
		_ = tRepos.Users.Delete(ctx, usr.ID)
	}

	// Cleanup
	err = tRepos.Users.DeleteAll(ctx)
	require.NoError(t, err)
}

func TestUserRepo_Update(t *testing.T) {
	user, err := tRepos.Users.Create(context.Background(), userFactory())
	require.NoError(t, err)

	updateData := UserUpdate{
		Name:  fk.Str(10),
		Email: fk.Email(),
	}

	// Update
	err = tRepos.Users.Update(context.Background(), user.ID, updateData)
	require.NoError(t, err)

	// Validate
	updated, err := tRepos.Users.GetOneID(context.Background(), user.ID)
	require.NoError(t, err)
	assert.NotEqual(t, user.Name, updated.Name)
	assert.NotEqual(t, user.Email, updated.Email)
}

func TestUserRepo_Delete(t *testing.T) {
	// Create 10 Users
	for i := 0; i < 10; i++ {
		user := userFactory()
		ctx := context.Background()
		_, _ = tRepos.Users.Create(ctx, user)
	}

	// Delete all
	ctx := context.Background()
	allUsers, _ := tRepos.Users.GetAll(ctx)

	assert.NotEmpty(t, allUsers)
	err := tRepos.Users.DeleteAll(ctx)
	require.NoError(t, err)

	allUsers, _ = tRepos.Users.GetAll(ctx)
	assert.Empty(t, allUsers)
}

func TestUserRepo_GetSuperusers(t *testing.T) {
	// Create 10 Users
	superuser := 0
	users := 0

	for i := 0; i < 10; i++ {
		user := userFactory()
		ctx := context.Background()
		_, _ = tRepos.Users.Create(ctx, user)

		if user.IsSuperuser {
			superuser++
		} else {
			users++
		}
	}

	// Delete all
	ctx := context.Background()

	superUsers, err := tRepos.Users.GetSuperusers(ctx)
	require.NoError(t, err)

	for _, usr := range superUsers {
		assert.True(t, usr.IsSuperuser)
	}

	// Cleanup
	err = tRepos.Users.DeleteAll(ctx)
	require.NoError(t, err)
}

func TestUserRepo_NewAccountTheme(t *testing.T) {
	ctx := context.Background()
	for _, oidc := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "oidc"}[oidc], func(t *testing.T) {
			input := userFactory()
			var created UserOut
			var err error
			if oidc {
				input.Password = nil
				created, err = tRepos.Users.CreateWithOIDC(ctx, input, "https://issuer.example", uuid.NewString())
			} else {
				created, err = tRepos.Users.Create(ctx, input)
			}
			require.NoError(t, err)
			t.Cleanup(func() { _ = tRepos.Users.Delete(ctx, created.ID) })
			settings, err := tRepos.Users.GetSettings(ctx, created.ID)
			require.NoError(t, err)
			assert.Equal(t, map[string]interface{}{"theme": "cinematic"}, settings)
		})
	}
}

func TestUserRepo_OIDCLinkPreservesSettings(t *testing.T) {
	ctx := context.Background()
	for _, settings := range []map[string]interface{}{nil, {}, {"theme": "light", "showDetails": false}} {
		created, err := tRepos.Users.Create(ctx, userFactory())
		require.NoError(t, err)
		t.Cleanup(func() { _ = tRepos.Users.Delete(ctx, created.ID) })
		require.NoError(t, tRepos.Users.SetSettings(ctx, created.ID, settings))
		subject := uuid.NewString()
		require.NoError(t, tRepos.Users.SetOIDCIdentity(ctx, created.ID, "https://issuer.example", subject))
		found, err := tRepos.Users.GetOneOIDC(ctx, "https://issuer.example", subject)
		require.NoError(t, err)
		assert.Equal(t, created.ID, found.ID)
		actual, err := tRepos.Users.GetSettings(ctx, found.ID)
		require.NoError(t, err)
		assert.Equal(t, settings, actual)
	}
}
