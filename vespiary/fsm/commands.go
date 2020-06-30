package fsm

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/golang/protobuf/proto"

	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/vespiary/vespiary/audit"
	"github.com/vx-labs/wasp/cluster/raft"
)

type State interface {
	DeleteDevice(id, owner string) error
	CreateDevice(device *api.Device) error
	EnableDevice(id, owner string) error
	DisableDevice(id, owner string) error
	ChangeDevicePassword(id, owner, password string) error
	CreateAccount(account *api.Account) error
	DeleteAccount(id string) error
}

func decode(payload []byte) ([]*StateTransition, error) {
	format := StateTransitionSet{}
	err := proto.Unmarshal(payload, &format)
	if err != nil {
		return nil, err
	}
	return format.Events, nil
}
func encode(events ...*StateTransition) ([]byte, error) {
	format := StateTransitionSet{
		Events: events,
	}
	return proto.Marshal(&format)
}

func NewFSM(id uint64, state State, commandsCh chan raft.Command, recorder audit.Recorder) *FSM {
	return &FSM{id: id, state: state, commandsCh: commandsCh, recorder: recorder}
}

type FSM struct {
	id         uint64
	state      State
	commandsCh chan raft.Command
	recorder   audit.Recorder
}

func (f *FSM) record(ctx context.Context, events ...*StateTransition) error {
	var err error
	for _, event := range events {
		switch event := event.GetEvent().(type) {
		case *StateTransition_AccountCreated:
			input := event.AccountCreated
			tenant := input.ID
			err = f.recorder.RecordEvent(tenant, audit.AccountCreated, map[string]string{})
		case *StateTransition_AccountDeleted:
			input := event.AccountDeleted
			tenant := input.ID
			err = f.recorder.RecordEvent(tenant, audit.AccountDeleted, map[string]string{})
		case *StateTransition_DeviceCreated:
			input := event.DeviceCreated
			tenant := input.Owner
			err = f.recorder.RecordEvent(tenant, audit.DeviceCreated, map[string]string{
				"device_id": input.ID,
			})
		case *StateTransition_DeviceDeleted:
			input := event.DeviceDeleted
			tenant := input.Owner
			err = f.recorder.RecordEvent(tenant, audit.DeviceDeleted, map[string]string{
				"device_id": input.ID,
			})
		case *StateTransition_DeviceEnabled:
			input := event.DeviceEnabled
			tenant := input.Owner
			err = f.recorder.RecordEvent(tenant, audit.DeviceEnabled, map[string]string{
				"device_id": input.ID,
			})
		case *StateTransition_DeviceDisabled:
			input := event.DeviceDisabled
			tenant := input.Owner
			err = f.recorder.RecordEvent(tenant, audit.DeviceDisabled, map[string]string{
				"device_id": input.ID,
			})
		case *StateTransition_DevicePasswordChanged:
			input := event.DevicePasswordChanged
			tenant := input.Owner
			err = f.recorder.RecordEvent(tenant, audit.DevicePasswordChanged, map[string]string{
				"device_id": input.ID,
			})
		case *StateTransition_PeerLost:
		}
		if err != nil {
			log.Println(err)
			// Do not fail if audit recording fails
			return nil
		}
	}
	return nil
}

func (f *FSM) commit(ctx context.Context, events ...*StateTransition) error {
	payload, err := encode(events...)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out := make(chan error)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case f.commandsCh <- raft.Command{Ctx: ctx, ErrCh: out, Payload: payload}:
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-out:
			if err == nil {
				f.record(ctx, events...)
			}
			return err
		}
	}
}

func (f *FSM) CreateAccount(ctx context.Context, name string, principals, deviceUsernames []string) (string, error) {
	if name == "" {
		return "", errors.New("account must have a name")
	}
	id := uuid.New().String()
	now := time.Now().UnixNano()

	return id, f.commit(ctx, &StateTransition{Event: &StateTransition_AccountCreated{
		AccountCreated: &AccountCreated{
			ID:              id,
			Name:            name,
			Principals:      principals,
			DeviceUsernames: deviceUsernames,
			CreatedAt:       now,
		},
	}})
}
func (f *FSM) DeleteAccount(ctx context.Context, id string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_AccountDeleted{
		AccountDeleted: &AccountDeleted{
			ID: id,
		},
	}})
}
func (f *FSM) DeleteDevice(ctx context.Context, id, owner string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_DeviceDeleted{
		DeviceDeleted: &DeviceDeleted{
			ID:    id,
			Owner: owner,
		},
	}})
}
func (f *FSM) DisableDevice(ctx context.Context, id, owner string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_DeviceDisabled{
		DeviceDisabled: &DeviceDisabled{
			ID:    id,
			Owner: owner,
		},
	}})
}
func (f *FSM) EnableDevice(ctx context.Context, id, owner string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_DeviceEnabled{
		DeviceEnabled: &DeviceEnabled{
			ID:    id,
			Owner: owner,
		},
	}})
}
func (f *FSM) ChangeDevicePassword(ctx context.Context, id, owner, password string) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_DevicePasswordChanged{
		DevicePasswordChanged: &DevicePasswordChanged{
			ID:    id,
			Owner: owner,
		},
	}})
}
func (f *FSM) CreateDevice(ctx context.Context, owner, name, password string, active bool) (string, error) {
	id := uuid.New().String()
	now := time.Now().UnixNano()
	return id, f.commit(ctx, &StateTransition{Event: &StateTransition_DeviceCreated{
		DeviceCreated: &DeviceCreated{
			ID:        id,
			Owner:     owner,
			Name:      name,
			CreatedAt: now,
			Password:  password,
			Active:    active,
		},
	}})
}

func (f *FSM) Shutdown(ctx context.Context) error {
	return f.commit(ctx, &StateTransition{Event: &StateTransition_PeerLost{
		PeerLost: &PeerLost{
			Peer: f.id,
		},
	}})
}

func (f *FSM) Apply(index uint64, b []byte) error {
	events, err := decode(b)
	if err != nil {
		return err
	}
	for _, event := range events {
		switch event := event.GetEvent().(type) {
		case *StateTransition_AccountCreated:
			in := event.AccountCreated
			err = f.state.CreateAccount(&api.Account{
				ID:              in.ID,
				Name:            in.Name,
				Principals:      in.DeviceUsernames,
				DeviceUsernames: in.DeviceUsernames,
				CreatedAt:       in.CreatedAt,
			})
		case *StateTransition_AccountDeleted:
			err = f.state.DeleteAccount(event.AccountDeleted.ID)
		case *StateTransition_DeviceCreated:
			in := event.DeviceCreated
			err = f.state.CreateDevice(&api.Device{
				ID:        in.ID,
				Owner:     in.Owner,
				Name:      in.Name,
				Active:    in.Active,
				CreatedAt: in.CreatedAt,
				Password:  in.Password,
			})
		case *StateTransition_DeviceDeleted:
			in := event.DeviceDeleted
			err = f.state.DeleteDevice(in.ID, in.Owner)
		case *StateTransition_DeviceEnabled:
			in := event.DeviceEnabled
			err = f.state.EnableDevice(in.ID, in.Owner)
		case *StateTransition_DeviceDisabled:
			in := event.DeviceDisabled
			err = f.state.DisableDevice(in.ID, in.Owner)
		case *StateTransition_DevicePasswordChanged:
			in := event.DevicePasswordChanged
			err = f.state.ChangeDevicePassword(in.ID, in.Owner, in.Password)
		default:
		}
		if err != nil {
			return err
		}
	}
	return nil
}
