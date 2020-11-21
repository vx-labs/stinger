package state

import (
	"errors"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/vx-labs/vespiary/vespiary/api"
)

var (
	ErrApplicationProfileNotFound = errors.New("application profile not found")
)

type ApplicationProfilesState interface {
	Create(application *api.ApplicationProfile) error
	ByID(id string) (*api.ApplicationProfile, error)
	ByAccountID(id, accountID string) (*api.ApplicationProfile, error)
	ListByAccountID(accountID string) ([]*api.ApplicationProfile, error)
	ListByApplicationID(applicationID string) ([]*api.ApplicationProfile, error)
	ListByApplicationIDAndAccountID(applicationID, accountID string) ([]*api.ApplicationProfile, error)
	Delete(id string) error
}

func applicationProfilesTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: applicationProfilesTable,
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
			"id_and_application_id_and_account_id": {
				Name: "id_and_application_id_and_account_id",
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "ID",
						},
						&memdb.StringFieldIndex{
							Field: "ApplicationID",
						},
						&memdb.StringFieldIndex{
							Field: "AccountID",
						},
					},
				},
				Unique:       true,
				AllowMissing: false,
			},
			"id_and_account_id": {
				Name: "id_and_account_id",
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "ID",
						},
						&memdb.StringFieldIndex{
							Field: "AccountID",
						},
					},
				},
				Unique:       true,
				AllowMissing: false,
			},
			"application_id_and_account_id": {
				Name: "application_id_and_account_id",
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "ApplicationID",
						},
						&memdb.StringFieldIndex{
							Field: "AccountID",
						},
					},
				},
				Unique:       false,
				AllowMissing: false,
			},
			"application_id": {
				Name: "application_id",
				Indexer: &memdb.StringFieldIndex{
					Field: "ApplicationID",
				},
				Unique:       false,
				AllowMissing: false,
			},
			"account_id": {
				Name: "account_id",
				Indexer: &memdb.StringFieldIndex{
					Field: "AccountID",
				},
				Unique:       false,
				AllowMissing: false,
			},
		},
	}
}

type applicationProfileState struct {
	db *memdb.MemDB
}

func (s *applicationProfileState) tableName() string { return applicationProfilesTable }

func newApplicationProfilesState(db *memdb.MemDB) ApplicationProfilesState {
	return &applicationProfileState{db: db}
}

func (s *applicationProfileState) Create(application *api.ApplicationProfile) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Insert(s.tableName(), application)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *applicationProfileState) ByID(id string) (*api.ApplicationProfile, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "id", id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrApplicationProfileNotFound
	}
	return v.(*api.ApplicationProfile), nil
}
func (s *applicationProfileState) ByAccountID(id, accountID string) (*api.ApplicationProfile, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "id_and_account_id", id, accountID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrApplicationProfileNotFound
	}
	return v.(*api.ApplicationProfile), nil
}
func (s *applicationProfileState) consumeIterator(iterator memdb.ResultIterator) ([]*api.ApplicationProfile, error) {
	out := make([]*api.ApplicationProfile, 0)
	for {
		v := iterator.Next()
		if v == nil {
			break
		}
		out = append(out, v.(*api.ApplicationProfile))
	}
	return out, nil
}
func (s *applicationProfileState) ListByAccountID(accountID string) ([]*api.ApplicationProfile, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "account_id", accountID)
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}
func (s *applicationProfileState) ListByApplicationID(applicationID string) ([]*api.ApplicationProfile, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "application_id", applicationID)
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}
func (s *applicationProfileState) ListByApplicationIDAndAccountID(applicationID, accountID string) ([]*api.ApplicationProfile, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "application_id_and_account_id", applicationID, accountID)
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}

func (s *applicationProfileState) delete(tx *memdb.Txn, id string) error {
	err := tx.Delete(s.tableName(), &api.ApplicationProfile{ID: id})
	if err != nil {
		if err == memdb.ErrNotFound {
			return ErrApplicationProfileNotFound
		}
		return err
	}
	tx.Commit()
	return nil
}

func (s *applicationProfileState) Delete(id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	return s.delete(tx, id)
}
