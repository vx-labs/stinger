package vespiary

import (
	"context"

	"github.com/vx-labs/vespiary/vespiary/api"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FSM interface {
	CreateDevice(ctx context.Context, owner, name, password string, active bool) (string, error)
	DeleteDevice(ctx context.Context, id, owner string) error
	EnableDevice(ctx context.Context, id, owner string) error
	DisableDevice(ctx context.Context, id, owner string) error
	ChangeDevicePassword(ctx context.Context, id, owner, password string) error
	CreateAccount(ctx context.Context, name string, principals, deviceUsernames []string) (string, error)
	DeleteAccount(ctx context.Context, id string) error
}

type State interface {
	DevicesByOwner(owner string) ([]*api.Device, error)
	DeviceByID(owner, id string) (*api.Device, error)
	DeviceByName(owner, name string) (*api.Device, error)
	ListAccounts() ([]*api.Account, error)
	AccountByName(name string) (*api.Account, error)
	AccountByID(id string) (*api.Account, error)
	AccountByDeviceUsername(deviceUsername string) (*api.Account, error)
	AccountByPrincipal(principal string) (*api.Account, error)
}

func NewServer(fsm FSM, state State) *server {
	return &server{
		fsm:   fsm,
		state: state,
	}
}

type server struct {
	fsm   FSM
	state State
}

func (s *server) Serve(grpcServer *grpc.Server) {
	api.RegisterVespiaryServer(grpcServer, s)
}

func (s *server) CreateDevice(ctx context.Context, input *api.CreateDeviceRequest) (*api.CreateDeviceResponse, error) {
	_, err := s.state.AccountByID(input.Owner)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	_, err = s.state.DeviceByName(input.Owner, input.Name)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "device already exists")
	}
	id, err := s.fsm.CreateDevice(ctx, input.Owner, input.Name, fingerprintString(input.Password), input.Active)
	if err != nil {
		return nil, err
	}
	return &api.CreateDeviceResponse{
		ID: id,
	}, nil
}
func (s *server) DeleteDevice(ctx context.Context, input *api.DeleteDeviceRequest) (*api.DeleteDeviceResponse, error) {
	_, err := s.state.DeviceByID(input.Owner, input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "device does not exist")
	}
	err = s.fsm.DeleteDevice(ctx, input.ID, input.Owner)
	if err != nil {
		return nil, err
	}
	return &api.DeleteDeviceResponse{
		ID: input.ID,
	}, nil
}
func (s *server) EnableDevice(ctx context.Context, input *api.EnableDeviceRequest) (*api.EnableDeviceResponse, error) {
	_, err := s.state.DeviceByID(input.Owner, input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "device does not exist")
	}
	err = s.fsm.EnableDevice(ctx, input.ID, input.Owner)
	if err != nil {
		return nil, err
	}
	return &api.EnableDeviceResponse{}, nil
}
func (s *server) DisableDevice(ctx context.Context, input *api.DisableDeviceRequest) (*api.DisableDeviceResponse, error) {
	_, err := s.state.DeviceByID(input.Owner, input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "device does not exist")
	}
	err = s.fsm.DisableDevice(ctx, input.ID, input.Owner)
	if err != nil {
		return nil, err
	}
	return &api.DisableDeviceResponse{}, nil
}

func (s *server) ListDevices(ctx context.Context, input *api.ListDevicesRequest) (*api.ListDevicesResponse, error) {
	_, err := s.state.AccountByID(input.Owner)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	devices, err := s.state.DevicesByOwner(input.Owner)
	if err != nil {
		return nil, err
	}
	return &api.ListDevicesResponse{Devices: devices}, nil
}
func (s *server) GetDevice(ctx context.Context, input *api.GetDeviceRequest) (*api.GetDeviceResponse, error) {
	device, err := s.state.DeviceByID(input.Owner, input.ID)
	if err != nil {
		return nil, err
	}
	return &api.GetDeviceResponse{Device: device}, nil
}
func (s *server) ChangeDevicePassword(ctx context.Context, input *api.ChangeDevicePasswordRequest) (*api.ChangeDevicePasswordResponse, error) {
	_, err := s.state.AccountByID(input.Owner)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	err = s.fsm.ChangeDevicePassword(ctx, input.ID, input.Owner, fingerprintString(input.NewPassword))
	if err != nil {
		return nil, err
	}
	return &api.ChangeDevicePasswordResponse{}, nil
}
func (s *server) CreateAccount(ctx context.Context, input *api.CreateAccountRequest) (*api.CreateAccountResponse, error) {
	_, err := s.state.AccountByName(input.Name)
	if err == nil {
		return nil, status.Error(codes.AlreadyExists, "account already exists")
	}
	id, err := s.fsm.CreateAccount(ctx, input.Name, input.Principals, input.DeviceUsernames)
	if err != nil {
		return nil, err
	}
	return &api.CreateAccountResponse{
		ID: id,
	}, nil

}
func (s *server) DeleteAccount(ctx context.Context, input *api.DeleteAccountRequest) (*api.DeleteAccountResponse, error) {
	_, err := s.state.AccountByID(input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	err = s.fsm.DeleteAccount(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	return &api.DeleteAccountResponse{}, nil

}

func (s *server) ListAccounts(ctx context.Context, input *api.ListAccountsRequest) (*api.ListAccountsResponse, error) {
	accounts, err := s.state.ListAccounts()
	if err != nil {
		return nil, err
	}
	return &api.ListAccountsResponse{Accounts: accounts}, nil

}
func (s *server) GetAccountByPrincipal(ctx context.Context, input *api.GetAccountByPrincipalRequest) (*api.GetAccountByPrincipalResponse, error) {
	accounts, err := s.state.AccountByPrincipal(input.Principal)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountByPrincipalResponse{Account: accounts}, nil

}
func (s *server) GetAccountByDeviceUsername(ctx context.Context, input *api.GetAccountByDeviceUsernameRequest) (*api.GetAccountByDeviceUsernameResponse, error) {
	accounts, err := s.state.AccountByDeviceUsername(input.Username)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountByDeviceUsernameResponse{Account: accounts}, nil

}
