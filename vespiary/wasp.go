package vespiary

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/vx-labs/wasp/wasp/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultMountPoint = "_default"

func fingerprintBytes(buf []byte) string {
	sum := sha256.Sum256(buf)
	return fmt.Sprintf("%x", sum)
}

func fingerprintString(buf string) string {
	return fingerprintBytes([]byte(buf))
}

type WaspAuthenticationServer struct {
	fsm   FSM
	state State
}

func NewWaspAuthenticationServer(fsm FSM, state State) *WaspAuthenticationServer {
	return &WaspAuthenticationServer{
		fsm:   fsm,
		state: state,
	}
}

func (s *WaspAuthenticationServer) Serve(grpcServer *grpc.Server) {
	auth.RegisterAuthenticationServer(grpcServer, s)
}

func (s *WaspAuthenticationServer) AuthenticateMQTTClient(ctx context.Context, input *auth.WaspAuthenticationRequest) (*auth.WaspAuthenticationResponse, error) {
	account, err := s.state.AccountByDeviceUsername(string(input.MQTT.Username))
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid username or password")
	}
	device, err := s.state.DeviceByName(account.ID, string(input.MQTT.ClientID))
	if err != nil {
		s.fsm.CreateDevice(ctx, string(input.MQTT.Username), string(input.MQTT.ClientID), fingerprintBytes(input.MQTT.Password), false)
		return nil, status.Error(codes.InvalidArgument, "invalid username or password")
	}
	if device.Active && device.Password == fingerprintBytes(input.MQTT.Password) {
		return &auth.WaspAuthenticationResponse{
			ID:         device.ID,
			MountPoint: device.Owner,
		}, nil
	}
	return nil, status.Error(codes.InvalidArgument, "device is disabled or password wrong")
}
