package main

import (
	"context"
	"errors"

	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/audit"
	"github.com/vx-labs/wasp/rpc"
	"go.uber.org/zap"
)

func getAuditRecorder(ctx context.Context, rpcDialer rpc.Dialer, config *viper.Viper, logger *zap.Logger) (audit.Recorder, error) {
	provider := config.GetString("audit-recorder")
	switch provider {
	case "none":
		return audit.NoneRecorder(), nil
	case "grpc":
		remote, err := rpcDialer(config.GetString("audit-recorder-grpc-address"))
		if err != nil {
			return nil, err
		}
		return audit.GRPCRecorder(remote, logger), nil
	default:
		return nil, errors.New("unknown authentication provided")
	}
}
