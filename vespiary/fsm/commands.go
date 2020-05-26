package fsm

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/golang/protobuf/proto"

	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/wasp/cluster/raft"
)

type State interface {
	DeleteDevice(id, owner string) error
	CreateDevice(device *api.Device) error
	EnableDevice(id, owner string) error
	DisableDevice(id, owner string) error
	ChangeDevicePassword(id, owner, password string) error
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

func NewFSM(id uint64, state State, commandsCh chan raft.Command) *FSM {
	return &FSM{id: id, state: state, commandsCh: commandsCh}
}

type FSM struct {
	id         uint64
	state      State
	commandsCh chan raft.Command
}

func (f *FSM) commit(ctx context.Context, payload []byte) error {
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
			return err
		}
	}
}

func (f *FSM) DeleteDevice(ctx context.Context, id, owner string) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_DeviceDeleted{
		DeviceDeleted: &DeviceDeleted{
			ID:    id,
			Owner: owner,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) DisableDevice(ctx context.Context, id, owner string) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_DeviceDisabled{
		DeviceDisabled: &DeviceDisabled{
			ID:    id,
			Owner: owner,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) EnableDevice(ctx context.Context, id, owner string) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_DeviceEnabled{
		DeviceEnabled: &DeviceEnabled{
			ID:    id,
			Owner: owner,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) ChangeDevicePassword(ctx context.Context, id, owner, password string) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_DevicePasswordChanged{
		DevicePasswordChanged: &DevicePasswordChanged{
			ID:    id,
			Owner: owner,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}
func (f *FSM) CreateDevice(ctx context.Context, owner, name, password string, active bool) (string, error) {
	id := uuid.New().String()
	now := time.Now().UnixNano()
	payload, err := encode(&StateTransition{Event: &StateTransition_DeviceCreated{
		DeviceCreated: &DeviceCreated{
			ID:        id,
			Owner:     owner,
			Name:      name,
			CreatedAt: now,
			Password:  password,
			Active:    active,
		},
	}})
	if err != nil {
		return "", err
	}
	return id, f.commit(ctx, payload)
}

func (f *FSM) Shutdown(ctx context.Context) error {
	payload, err := encode(&StateTransition{Event: &StateTransition_PeerLost{
		PeerLost: &PeerLost{
			Peer: f.id,
		},
	}})
	if err != nil {
		return err
	}
	return f.commit(ctx, payload)
}

func (f *FSM) Apply(index uint64, b []byte) error {
	events, err := decode(b)
	if err != nil {
		return err
	}
	for _, event := range events {
		switch event := event.GetEvent().(type) {
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
