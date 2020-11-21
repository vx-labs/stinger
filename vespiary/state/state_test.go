package state

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/vx-labs/vespiary/vespiary/api"
)

func Test_memDBStore(t *testing.T) {
	store := NewStateStore()
	id := "4bffa69d-6a09-4b4c-ade8-197fa75b7d8e"
	owner := "491c17f0-eb84-4bdf-85ac-411fdbe928ca"

	t.Run("create account", func(t *testing.T) {
		require.NoError(t, store.CreateAccount(&api.Account{
			ID:   owner,
			Name: "my_account",
		}))
	})
	t.Run("create device", func(t *testing.T) {
		require.NoError(t, store.CreateDevice(&api.Device{
			ID:     id,
			Owner:  owner,
			Name:   "my_device",
			Active: true,
		}))
	})
	t.Run("delete device", func(t *testing.T) {
		require.NoError(t, store.DeleteDevice(id, owner))
	})
	t.Run("delete missing device", func(t *testing.T) {
		require.Error(t, store.DeleteDevice(id, owner))
	})
}
