package vespiary

import (
	"context"

	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/vespiary/vespiary/state"
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
	AddDeviceUsername(ctx context.Context, accountID string, deviceUsername string) error
	RemoveDeviceUsername(ctx context.Context, accountID string, deviceUsername string) error
	CreateApplication(ctx context.Context, accountID, name string) (string, error)
	DeleteApplication(ctx context.Context, id string) error
	CreateApplicationProfile(ctx context.Context, applicationID, accountID, name, password string) (string, error)
	DeleteApplicationProfile(ctx context.Context, id string) error
}

func NewServer(fsm FSM, state state.Store) *server {
	return &server{
		fsm:   fsm,
		state: state,
	}
}

type server struct {
	fsm   FSM
	state state.Store
}

func (s *server) Serve(grpcServer *grpc.Server) {
	api.RegisterVespiaryServer(grpcServer, s)
}

func (s *server) CreateDevice(ctx context.Context, input *api.CreateDeviceRequest) (*api.CreateDeviceResponse, error) {
	_, err := s.state.Accounts().ByID(input.Owner)
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
	_, err := s.state.Accounts().ByID(input.Owner)
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
	_, err := s.state.Accounts().ByID(input.Owner)
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
	_, err := s.state.Accounts().ByName(input.Name)
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
	_, err := s.state.Accounts().ByID(input.ID)
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
	accounts, err := s.state.Accounts().All()
	if err != nil {
		return nil, err
	}
	return &api.ListAccountsResponse{Accounts: accounts}, nil

}
func (s *server) GetAccountByPrincipal(ctx context.Context, input *api.GetAccountByPrincipalRequest) (*api.GetAccountByPrincipalResponse, error) {
	accounts, err := s.state.Accounts().ByPrincipal(input.Principal)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountByPrincipalResponse{Account: accounts}, nil

}
func (s *server) GetAccountByDeviceUsername(ctx context.Context, input *api.GetAccountByDeviceUsernameRequest) (*api.GetAccountByDeviceUsernameResponse, error) {
	accounts, err := s.state.Accounts().ByDeviceUsername(input.Username)
	if err != nil {
		return nil, err
	}
	return &api.GetAccountByDeviceUsernameResponse{Account: accounts}, nil

}

func (s *server) AddAccountDeviceUsername(ctx context.Context, input *api.AddAccountDeviceUsernameRequest) (*api.AddAccountDeviceUsernameResponse, error) {
	_, err := s.state.Accounts().ByID(input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	err = s.fsm.AddDeviceUsername(ctx, input.ID, input.Username)
	if err != nil {
		return nil, err
	}
	return &api.AddAccountDeviceUsernameResponse{}, nil

}
func (s *server) RemoveAccountDeviceUsername(ctx context.Context, input *api.RemoveAccountDeviceUsernameRequest) (*api.RemoveAccountDeviceUsernameResponse, error) {
	_, err := s.state.Accounts().ByID(input.ID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "account does not exist")
	}
	err = s.fsm.RemoveDeviceUsername(ctx, input.ID, input.Username)
	if err != nil {
		return nil, err
	}
	return &api.RemoveAccountDeviceUsernameResponse{}, nil

}

func (s *server) CreateApplication(ctx context.Context, input *api.CreateApplicationRequest) (*api.CreateApplicationResponse, error) {
	id, err := s.fsm.CreateApplication(ctx, input.AccountID, input.Name)
	if err != nil {
		return nil, err
	}
	return &api.CreateApplicationResponse{
		ID: id,
	}, nil
}
func (s *server) DeleteApplication(ctx context.Context, input *api.DeleteApplicationRequest) (*api.DeleteApplicationResponse, error) {
	err := s.fsm.DeleteApplication(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	return &api.DeleteApplicationResponse{}, nil
}
func (s *server) ListApplications(ctx context.Context, input *api.ListApplicationsRequest) (*api.ListApplicationsResponse, error) {
	out, err := s.state.Applications().All()
	if err != nil {
		return nil, err
	}
	return &api.ListApplicationsResponse{Applications: out}, nil
}
func (s *server) GetApplication(ctx context.Context, input *api.GetApplicationRequest) (*api.GetApplicationResponse, error) {
	out, err := s.state.Applications().ByID(input.Id)
	if err != nil {
		return nil, err
	}
	return &api.GetApplicationResponse{Application: out}, nil
}
func (s *server) GetApplicationByAccountID(ctx context.Context, input *api.GetApplicationByAccountIDRequest) (*api.GetApplicationByAccountIDResponse, error) {
	out, err := s.state.Applications().ByAccountID(input.Id, input.AccountID)
	if err != nil {
		return nil, err
	}
	return &api.GetApplicationByAccountIDResponse{Application: out}, nil
}
func (s *server) GetApplicationByName(ctx context.Context, input *api.GetApplicationByNameRequest) (*api.GetApplicationByNameResponse, error) {
	out, err := s.state.Applications().ByNameAndAccountID(input.Name, input.AccountID)
	if err != nil {
		return nil, err
	}
	return &api.GetApplicationByNameResponse{Application: out}, nil
}
func (s *server) ListApplicationsByAccountID(ctx context.Context, input *api.ListApplicationsByAccountIDRequest) (*api.ListApplicationsByAccountIDResponse, error) {
	out, err := s.state.Applications().ListByAccountID(input.AccountID)
	if err != nil {
		return nil, err
	}
	return &api.ListApplicationsByAccountIDResponse{Applications: out}, nil
}
func (s *server) CreateApplicationProfile(ctx context.Context, input *api.CreateApplicationProfileRequest) (*api.CreateApplicationProfileResponse, error) {
	id, err := s.fsm.CreateApplicationProfile(ctx, input.ApplicationID, input.AccountID, input.Name, input.Password)
	if err != nil {
		return nil, err
	}
	return &api.CreateApplicationProfileResponse{
		ID: id,
	}, nil
}
func (s *server) ListApplicationProfiles(ctx context.Context, input *api.ListApplicationProfilesRequest) (*api.ListApplicationProfilesResponse, error) {
	out, err := s.state.ApplicationProfiles().All()
	if err != nil {
		return nil, err
	}
	return &api.ListApplicationProfilesResponse{ApplicationProfiles: out}, nil
}
func (s *server) ListApplicationProfilesByAccountID(ctx context.Context, input *api.ListApplicationProfilesByAccountIDRequest) (*api.ListApplicationProfilesByAccountIDResponse, error) {
	out, err := s.state.ApplicationProfiles().ListByAccountID(input.AccountID)
	if err != nil {
		return nil, err
	}
	return &api.ListApplicationProfilesByAccountIDResponse{ApplicationProfiles: out}, nil
}
func (s *server) ListApplicationProfilesByApplication(ctx context.Context, input *api.ListApplicationProfilesByApplicationRequest) (*api.ListApplicationProfilesByApplicationResponse, error) {
	out, err := s.state.ApplicationProfiles().ListByApplicationIDAndAccountID(input.ApplicationID, input.AccountID)
	if err != nil {
		return nil, err
	}
	return &api.ListApplicationProfilesByApplicationResponse{ApplicationProfiles: out}, nil
}

func (s *server) DeleteApplicationProfile(ctx context.Context, input *api.DeleteApplicationProfileRequest) (*api.DeleteApplicationProfileResponse, error) {
	err := s.fsm.DeleteApplicationProfile(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	return &api.DeleteApplicationProfileResponse{}, nil
}
