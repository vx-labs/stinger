package fsm

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/golang/protobuf/proto"

	"github.com/vx-labs/cluster/raft"
	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/vespiary/vespiary/audit"
	"github.com/vx-labs/vespiary/vespiary/state"
)

var (
	ErrAccountDoesNotExist  = errors.New("account does not exist")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrPasswordTooShort     = errors.New("password too short")
	ErrInvalidName          = errors.New("invalid name")
)

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

// https://stackoverflow.com/questions/32349807/how-can-i-generate-a-random-int-using-the-crypto-rand-package/32350135
func generateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func NewFSM(id uint64, state state.Store, commandsCh chan raft.Command, recorder audit.Recorder) *FSM {
	return &FSM{id: id, state: state, commandsCh: commandsCh, recorder: recorder}
}

type FSM struct {
	id         uint64
	state      state.Store
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
		case *StateTransition_AccountDeviceUsernameAdded:
			input := event.AccountDeviceUsernameAdded
			tenant := input.ID
			err = f.recorder.RecordEvent(tenant, audit.DevicePasswordChanged, map[string]string{
				"device_username": input.DeviceUsername,
			})
		case *StateTransition_AccountDeviceUsernameRemoved:
			input := event.AccountDeviceUsernameRemoved
			tenant := input.ID
			err = f.recorder.RecordEvent(tenant, audit.DevicePasswordChanged, map[string]string{
				"device_username": input.DeviceUsername,
			})
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
	_, err := f.state.Accounts().ByName(name)
	if err == nil {
		return "", ErrAccountAlreadyExists
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
func (f *FSM) AddDeviceUsername(ctx context.Context, accountID string, deviceUsername string) error {
	if _, err := f.state.Accounts().ByID(accountID); err != nil {
		return ErrAccountDoesNotExist
	}
	return f.commit(ctx, &StateTransition{Event: &StateTransition_AccountDeviceUsernameAdded{
		&AccountDeviceUsernameAdded{
			ID:             accountID,
			DeviceUsername: deviceUsername,
		},
	}})
}
func (f *FSM) RemoveDeviceUsername(ctx context.Context, accountID string, deviceUsername string) error {
	if _, err := f.state.Accounts().ByID(accountID); err != nil {
		return ErrAccountDoesNotExist
	}
	return f.commit(ctx, &StateTransition{Event: &StateTransition_AccountDeviceUsernameRemoved{
		&AccountDeviceUsernameRemoved{
			ID:             accountID,
			DeviceUsername: deviceUsername,
		},
	}})
}
func (f *FSM) DeleteAccount(ctx context.Context, id string) error {
	_, err := f.state.Accounts().ByID(id)
	if err != nil {
		return err
	}
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
			ID:       id,
			Owner:    owner,
			Password: password,
		},
	}})
}
func (f *FSM) CreateDevice(ctx context.Context, owner, name, password string, active bool) (string, error) {
	if _, err := f.state.Accounts().ByID(owner); err != nil {
		return "", ErrAccountDoesNotExist
	}
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
func (f *FSM) CreateApplication(ctx context.Context, accountID, name string) (string, error) {
	if len(name) == 0 {
		return "", errors.New("application must have a name")
	}
	_, err := f.state.Accounts().ByID(accountID)
	if err != nil {
		return "", err
	}
	id := uuid.New().String()
	now := time.Now().UnixNano()
	return id, f.commit(ctx, &StateTransition{Event: &StateTransition_ApplicationCreated{
		ApplicationCreated: &ApplicationCreated{
			ID:        id,
			AccountID: accountID,
			Name:      name,
			CreatedAt: now,
		},
	}})
}
func (f *FSM) DeleteApplication(ctx context.Context, id string) error {
	_, err := f.state.Applications().ByID(id)
	if err != nil {
		return err
	}
	return f.commit(ctx, &StateTransition{Event: &StateTransition_ApplicationDeleted{
		ApplicationDeleted: &ApplicationDeleted{
			ID: id,
		},
	}})
}
func (f *FSM) CreateApplicationProfile(ctx context.Context, applicationID, accountID, name, password string) (string, error) {
	if len(name) == 0 {
		return "", errors.New("application profile must have a name")
	}
	if len(password) < 8 {
		return "", ErrPasswordTooShort
	}

	id := uuid.New().String()
	now := time.Now().UnixNano()
	_, err := f.state.Applications().ByAccountID(applicationID, accountID)
	if err != nil {
		return "", err
	}

	salt, err := generateRandomBytes(256)
	if err != nil {
		return "", err
	}
	passwordFingerprint := sha256.Sum256(append([]byte(password), salt...))

	return id, f.commit(ctx, &StateTransition{Event: &StateTransition_ApplicationProfileCreated{
		ApplicationProfileCreated: &ApplicationProfileCreated{
			ID:                  id,
			ApplicationID:       applicationID,
			AccountID:           accountID,
			Name:                name,
			CreatedAt:           now,
			Enabled:             true,
			PasswordSalt:        salt,
			PasswordFingerprint: passwordFingerprint[:],
		},
	}})
}
func (f *FSM) DeleteApplicationProfile(ctx context.Context, id string) error {
	_, err := f.state.ApplicationProfiles().ByID(id)
	if err != nil {
		return err
	}

	return f.commit(ctx, &StateTransition{Event: &StateTransition_ApplicationProfileDeleted{
		ApplicationProfileDeleted: &ApplicationProfileDeleted{
			ID: id,
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
			err = f.state.Accounts().Create(&api.Account{
				ID:              in.ID,
				Name:            in.Name,
				Principals:      in.Principals,
				DeviceUsernames: in.DeviceUsernames,
				CreatedAt:       in.CreatedAt,
			})
		case *StateTransition_AccountDeleted:
			err = f.state.Accounts().Delete(event.AccountDeleted.ID)
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
		case *StateTransition_ApplicationCreated:
			in := event.ApplicationCreated
			err = f.state.Applications().Create(&api.Application{
				ID:        in.ID,
				Name:      in.Name,
				AccountID: in.AccountID,
			})
		case *StateTransition_ApplicationDeleted:
			in := event.ApplicationDeleted
			err = f.state.Applications().Delete(in.ID)
		case *StateTransition_ApplicationProfileCreated:
			in := event.ApplicationProfileCreated
			err = f.state.ApplicationProfiles().Create(&api.ApplicationProfile{
				ID:                  in.ID,
				ApplicationID:       in.ApplicationID,
				Name:                in.Name,
				AccountID:           in.AccountID,
				Enabled:             in.Enabled,
				PasswordFingerprint: in.PasswordFingerprint,
				PasswordSalt:        in.PasswordSalt,
			})
		case *StateTransition_ApplicationProfileDeleted:
			in := event.ApplicationProfileDeleted
			err = f.state.ApplicationProfiles().Delete(in.ID)
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
		case *StateTransition_AccountDeviceUsernameAdded:
			in := event.AccountDeviceUsernameAdded
			err = f.state.Accounts().AddDeviceUsername(in.ID, in.DeviceUsername)
		case *StateTransition_AccountDeviceUsernameRemoved:
			in := event.AccountDeviceUsernameRemoved
			err = f.state.Accounts().RemoveDeviceUsername(in.ID, in.DeviceUsername)
		default:
		}
		if err != nil {
			return err
		}
	}
	return nil
}
