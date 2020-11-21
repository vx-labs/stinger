package state

import (
	"errors"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/vx-labs/vespiary/vespiary/api"
)

var (
	ErrAccountNotFound = errors.New("account not found")
)

type AccountsState interface {
	Create(account *api.Account) error
	ByID(id string) (*api.Account, error)
	ByName(name string) (*api.Account, error)
	ByPrincipal(principal string) (*api.Account, error)
	ByDeviceUsername(deviceUsername string) (*api.Account, error)
	AddDeviceUsername(account string, username string) error
	RemoveDeviceUsername(account string, username string) error
	All() ([]*api.Account, error)
	Delete(id string) error
}

func accountsTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: accountsTable,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name: "id",
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
				Unique:       true,
				AllowMissing: false,
			},
			"name": {
				Name: "name",
				Indexer: &memdb.StringFieldIndex{
					Field: "Name",
				},
				Unique:       true,
				AllowMissing: false,
			},
			"principals": {
				Name: "principals",
				Indexer: &memdb.StringSliceFieldIndex{
					Field: "Principals",
				},

				Unique:       true,
				AllowMissing: true,
			},
			"deviceUsernames": {
				Name: "deviceUsernames",
				Indexer: &memdb.StringSliceFieldIndex{
					Field: "DeviceUsernames",
				},
				Unique:       true,
				AllowMissing: true,
			},
		},
	}
}

type accountsState struct {
	db *memdb.MemDB
}

func (s *accountsState) tableName() string { return accountsTable }

func newAccountsState(db *memdb.MemDB) AccountsState {
	return &accountsState{db: db}
}

func (s *accountsState) consumeIterator(iterator memdb.ResultIterator) ([]*api.Account, error) {
	out := make([]*api.Account, 0)
	for {
		v := iterator.Next()
		if v == nil {
			break
		}
		out = append(out, v.(*api.Account))
	}
	return out, nil
}

func (s *accountsState) Create(account *api.Account) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Insert(accountsTable, account)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *accountsState) ByID(id string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "id", id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrAccountNotFound
	}
	return v.(*api.Account), nil
}
func (s *accountsState) ByName(name string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "name", name)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrAccountNotFound
	}
	return v.(*api.Account), nil
}
func (s *accountsState) ByPrincipal(principal string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "principals", principal)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrAccountNotFound
	}
	return v.(*api.Account), nil
}
func (s *accountsState) accountByID(tx *memdb.Txn, id string) (*api.Account, error) {
	elt, err := tx.First(accountsTable, "id", id)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Account), nil
}

func (s *accountsState) RemoveDeviceUsername(accountID string, username string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	account, err := s.accountByID(tx, accountID)
	if err != nil {
		return ErrAccountDoesNotExist
	}
	newList := make([]string, 0)
	for idx := range account.DeviceUsernames {
		if username != account.DeviceUsernames[idx] {
			newList = append(newList, account.DeviceUsernames[idx])
		}
	}
	if len(newList) == len(account.DeviceUsernames) {
		return nil
	}
	account.DeviceUsernames = newList
	err = tx.Insert(accountsTable, account)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *accountsState) AddDeviceUsername(accountID string, username string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	account, err := s.accountByID(tx, accountID)
	if err != nil {
		return ErrAccountDoesNotExist
	}
	for idx := range account.DeviceUsernames {
		if username == account.DeviceUsernames[idx] {
			return nil
		}
	}
	account.DeviceUsernames = append(account.DeviceUsernames, username)
	err = tx.Insert(accountsTable, account)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *accountsState) ByDeviceUsername(deviceUsername string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(accountsTable, "deviceUsernames", deviceUsername)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Account), nil
}

func (s *accountsState) All() ([]*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "id")
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}

func (s *accountsState) delete(tx *memdb.Txn, id string) error {
	err := tx.Delete(accountsTable, &api.Account{ID: id})
	if err != nil {
		return err
	}
	_, err = tx.DeleteAll(devicesTable, "owner", id)
	if err != nil {
		return err
	}
	_, err = tx.DeleteAll(applicationProfilesTable, "account_id", id)
	if err != nil {
		return err
	}
	_, err = tx.DeleteAll(applicationsTable, "account_id", id)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *accountsState) Delete(id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	return s.delete(tx, id)
}
