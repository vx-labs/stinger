package vespiary

import (
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/proto"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/vx-labs/vespiary/vespiary/api"
)

const (
	devicesTable  = "devices"
	accountsTable = "accounts"
)

type memDBStore struct {
	db *memdb.MemDB
}

func NewStateStore() *memDBStore {
	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			devicesTable: {
				Name: devicesTable,
				Indexes: map[string]*memdb.IndexSchema{
					"id": {
						Name: "id",
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{
									Field: "ID",
								},
								&memdb.StringFieldIndex{
									Field: "Owner",
								},
							},
						},
						Unique:       true,
						AllowMissing: false,
					},
					"owner": {
						Name: "owner",
						Indexer: &memdb.StringFieldIndex{
							Field: "Owner",
						},
						Unique:       false,
						AllowMissing: false,
					},
					"name": {
						Name: "name",
						Indexer: &memdb.CompoundIndex{
							Indexes: []memdb.Indexer{
								&memdb.StringFieldIndex{
									Field: "Owner",
								},
								&memdb.StringFieldIndex{
									Field: "Name",
								},
							},
						},
						Unique:       true,
						AllowMissing: false,
					},
				},
			},
			accountsTable: {
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
			},
		},
	})
	if err != nil {
		panic(err)
	}
	s := &memDBStore{
		db: db,
	}
	return s
}

func (s *memDBStore) CreateDevice(device *api.Device) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Insert(devicesTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) CreateAccount(account *api.Account) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Insert(accountsTable, account)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DeleteDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Delete(devicesTable, &api.Device{ID: id, Owner: owner})
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DeleteAccount(id string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Delete(accountsTable, &api.Account{ID: id})
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DevicesByOwner(owner string) ([]*api.Device, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(devicesTable, "owner", owner)
	if err != nil {
		return nil, err
	}
	out := []*api.Device{}
	for elt := iterator.Next(); elt != nil; elt = iterator.Next() {
		out = append(out, elt.(*api.Device))
	}
	return out, nil
}
func (s *memDBStore) DeviceByName(owner, name string) (*api.Device, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(devicesTable, "name", owner, name)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Device), nil
}
func (s *memDBStore) AccountByName(name string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(accountsTable, "name", name)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Account), nil
}
func (s *memDBStore) DeviceByID(owner, id string) (*api.Device, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(devicesTable, "id", id, owner)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Device), nil
}
func (s *memDBStore) ListAccounts() ([]*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(accountsTable, "id")
	if err != nil {
		return nil, err
	}
	out := []*api.Account{}
	for elt := iterator.Next(); elt != nil; elt = iterator.Next() {
		out = append(out, elt.(*api.Account))
	}
	return out, nil
}

func (s *memDBStore) AccountByID(id string) (*api.Account, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(accountsTable, "id", id)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Account), nil
}
func (s *memDBStore) EnableDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(devicesTable, "id", id, owner)
	if err != nil {
		return err
	}
	if elt == nil {
		return errors.New("not found")
	}
	device := elt.(*api.Device)
	if device.Active {
		return nil
	}
	device.Active = true
	err = tx.Insert(devicesTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DisableDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(devicesTable, "id", id, owner)
	if err != nil {
		return err
	}
	if elt == nil {
		return errors.New("not found")
	}
	device := elt.(*api.Device)
	if !device.Active {
		return nil
	}
	device.Active = false
	err = tx.Insert(devicesTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *memDBStore) ChangeDevicePassword(id, owner, password string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(devicesTable, "id", id, owner)
	if err != nil {
		return err
	}
	if elt == nil {
		return errors.New("not found")
	}
	device := elt.(*api.Device)
	device.Password = password
	err = tx.Insert(devicesTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

type Dump struct {
	Accounts []byte
	Devices  []byte
}

func (s *memDBStore) Dump() ([]byte, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(devicesTable, "id")
	if err != nil {
		return nil, err
	}
	devices := []*api.Device{}
	for elt := iterator.Next(); elt != nil; elt = iterator.Next() {
		devices = append(devices, elt.(*api.Device))
	}
	devicesPayload, _ := proto.Marshal(&api.DeviceSet{Devices: devices})
	iterator, err = tx.Get(accountsTable, "id")
	if err != nil {
		return nil, err
	}
	accounts := []*api.Account{}
	for elt := iterator.Next(); elt != nil; elt = iterator.Next() {
		accounts = append(accounts, elt.(*api.Account))
	}
	accountsPayload, _ := proto.Marshal(&api.AccountSet{Accounts: accounts})
	return json.Marshal(&Dump{
		Devices:  devicesPayload,
		Accounts: accountsPayload,
	})
}

func (s *memDBStore) Load(buf []byte) error {
	set := api.DeviceSet{}
	err := proto.Unmarshal(buf, &set)
	if err != nil {
		return s.Loadv2(buf)
	}
	tx := s.db.Txn(true)
	_, err = tx.DeleteAll(devicesTable, "id")
	if err != nil {
		return err
	}

	for _, device := range set.Devices {
		err := tx.Insert(devicesTable, device)
		if err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) Loadv2(buf []byte) error {
	dump := Dump{}
	err := json.Unmarshal(buf, &dump)
	if err != nil {
		return err
	}
	tx := s.db.Txn(true)
	_, err = tx.DeleteAll(devicesTable, "id")
	if err != nil {
		return err
	}
	_, err = tx.DeleteAll(accountsTable, "id")
	if err != nil {
		return err
	}
	deviceSet := &api.DeviceSet{}
	accountSet := &api.AccountSet{}
	err = proto.Unmarshal(dump.Devices, deviceSet)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(dump.Accounts, accountSet)
	if err != nil {
		return err
	}
	for _, account := range accountSet.Accounts {
		err := tx.Insert(accountsTable, account)
		if err != nil {
			return err
		}
	}
	for _, device := range deviceSet.Devices {
		err := tx.Insert(devicesTable, device)
		if err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}
