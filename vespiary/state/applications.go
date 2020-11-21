package state

import (
	"errors"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/vx-labs/vespiary/vespiary/api"
)

var (
	ErrApplicationNotFound = errors.New("application not found")
)

type ApplicationsState interface {
	Create(application *api.Application) error
	ByID(id string) (*api.Application, error)
	ByNameAndAccountID(name, accountID string) (*api.Application, error)
	ByAccountID(id, accountID string) (*api.Application, error)
	All() ([]*api.Application, error)
	ListByAccountID(accountID string) ([]*api.Application, error)
	Delete(id string) error
}

func applicationTableSchema() *memdb.TableSchema {
	return &memdb.TableSchema{
		Name: applicationsTable,
		Indexes: map[string]*memdb.IndexSchema{
			"id": {
				Name: "id",
				Indexer: &memdb.StringFieldIndex{
					Field: "ID",
				},
				Unique:       true,
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
			"name_and_account_id": {
				Name: "name_and_account_id",
				Indexer: &memdb.CompoundIndex{
					Indexes: []memdb.Indexer{
						&memdb.StringFieldIndex{
							Field: "Name",
						},
						&memdb.StringFieldIndex{
							Field: "AccountID",
						},
					},
				},
				Unique:       true,
				AllowMissing: false,
			},
		},
	}
}

type applicationState struct {
	db *memdb.MemDB
}

func (s *applicationState) tableName() string { return applicationsTable }

func newApplicationsState(db *memdb.MemDB) ApplicationsState {
	return &applicationState{db: db}
}

func (s *applicationState) Create(application *api.Application) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Insert(s.tableName(), application)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *applicationState) ByID(id string) (*api.Application, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "id", id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrApplicationNotFound
	}
	return v.(*api.Application), nil
}
func (s *applicationState) ByNameAndAccountID(name, accountID string) (*api.Application, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "name_and_account_id", name, accountID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrApplicationNotFound
	}
	return v.(*api.Application), nil
}
func (s *applicationState) ByAccountID(id, accountID string) (*api.Application, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	v, err := tx.First(s.tableName(), "id_and_account_id", id, accountID)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrApplicationNotFound
	}
	return v.(*api.Application), nil
}
func (s *applicationState) consumeIterator(iterator memdb.ResultIterator) ([]*api.Application, error) {
	out := make([]*api.Application, 0)
	for {
		v := iterator.Next()
		if v == nil {
			break
		}
		out = append(out, v.(*api.Application))
	}
	return out, nil
}

func (s *applicationState) ListByAccountID(accountID string) ([]*api.Application, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "account_id", accountID)
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}

func (s *applicationState) All() ([]*api.Application, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(s.tableName(), "id")
	if err != nil {
		return nil, err
	}
	return s.consumeIterator(iterator)
}

func (s *applicationState) delete(tx *memdb.Txn, id string) error {
	err := tx.Delete(s.tableName(), &api.Application{ID: id})
	if err != nil {
		if err == memdb.ErrNotFound {
			return ErrApplicationNotFound
		}
		return err
	}
	_, err = tx.DeleteAll(applicationProfilesTable, "application_id", id)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *applicationState) Delete(id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	return s.delete(tx, id)
}
