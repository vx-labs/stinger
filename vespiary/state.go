package vespiary

import (
	"errors"

	"github.com/golang/protobuf/proto"

	memdb "github.com/hashicorp/go-memdb"
	"github.com/vx-labs/vespiary/vespiary/api"
)

const (
	memdbTable = "devices"
)

type memDBStore struct {
	db *memdb.MemDB
}

func NewStateStore() *memDBStore {
	db, err := memdb.NewMemDB(&memdb.DBSchema{
		Tables: map[string]*memdb.TableSchema{
			memdbTable: {
				Name: memdbTable,
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
	err := tx.Insert(memdbTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DeleteDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	err := tx.Delete(memdbTable, &api.Device{ID: id, Owner: owner})
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DevicesByOwner(owner string) ([]*api.Device, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(memdbTable, "owner", owner)
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
	elt, err := tx.First(memdbTable, "name", owner, name)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Device), nil
}
func (s *memDBStore) DeviceByID(owner, id string) (*api.Device, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	elt, err := tx.First(memdbTable, "id", id, owner)
	if err != nil {
		return nil, err
	}
	if elt == nil {
		return nil, errors.New("not found")
	}
	return elt.(*api.Device), nil
}
func (s *memDBStore) EnableDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(memdbTable, "id", id, owner)
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
	err = tx.Insert(memdbTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
func (s *memDBStore) DisableDevice(id, owner string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(memdbTable, "id", id, owner)
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
	err = tx.Insert(memdbTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *memDBStore) ChangeDevicePassword(id, owner, password string) error {
	tx := s.db.Txn(true)
	defer tx.Abort()
	elt, err := tx.First(memdbTable, "id", id, owner)
	if err != nil {
		return err
	}
	if elt == nil {
		return errors.New("not found")
	}
	device := elt.(*api.Device)
	device.Password = password
	err = tx.Insert(memdbTable, device)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}

func (s *memDBStore) Dump() ([]byte, error) {
	tx := s.db.Txn(false)
	defer tx.Abort()
	iterator, err := tx.Get(memdbTable, "id")
	if err != nil {
		return nil, err
	}
	devices := []*api.Device{}
	for elt := iterator.Next(); elt != nil; elt = iterator.Next() {
		devices = append(devices, elt.(*api.Device))
	}
	out := &api.DeviceSet{
		Devices: devices,
	}
	return proto.Marshal(out)
}

func (s *memDBStore) Load(buf []byte) error {
	tx := s.db.Txn(true)
	_, err := tx.DeleteAll(memdbTable, "id")
	if err != nil {
		return err
	}
	set := api.DeviceSet{}
	err = proto.Unmarshal(buf, &set)
	if err != nil {
		return err
	}
	for _, device := range set.Devices {
		err := tx.Insert(memdbTable, device)
		if err != nil {
			return err
		}
	}
	tx.Commit()
	return nil
}
