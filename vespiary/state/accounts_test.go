package state

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vx-labs/vespiary/vespiary/api"
)

func Test_memDBStore_Accounts(t *testing.T) {
	id := "4bffa69d-6a09-4b4c-ade8-197fa75b7d8e"

	t.Run("create-get-list-delete", func(t *testing.T) {
		store := newAccountsState(newDB())

		require.NoError(t, store.Create(&api.Account{
			ID:              id,
			Name:            "my_account",
			DeviceUsernames: []string{"username"},
			Principals:      []string{"principal"},
		}))
		app, err := store.ByID(id)
		require.NoError(t, err)
		require.NotNil(t, app)
		require.Equal(t, app.ID, id)
		require.Equal(t, app.Name, "my_account")

		app, err = store.ByDeviceUsername("username")
		require.NoError(t, err)
		require.NotNil(t, app)
		require.Equal(t, app.ID, id)
		require.Equal(t, app.Name, "my_account")

		apps, err := store.All()
		require.NoError(t, err)
		require.NotNil(t, app)
		require.Equal(t, 1, len(apps))
		require.Equal(t, apps[0].Name, "my_account")

		err = store.Delete(id)
		require.NoError(t, err)
		_, err = store.ByID(id)
		require.Equal(t, err, ErrAccountNotFound)
	})
}
