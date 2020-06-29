package audit

import (
	"context"
	"time"

	"github.com/vx-labs/vespiary/vespiary/api"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type grpcRecorder struct {
	ctx    context.Context
	logger *zap.Logger
	client api.VespiaryAuditRecorderClient
}

func (s *grpcRecorder) RecordEvent(tenant string, eventKind event, payload map[string]string) error {
	attributes := make([]*api.VespiaryEventAttribute, len(payload))
	idx := 0
	for key, value := range payload {
		attributes[idx] = &api.VespiaryEventAttribute{Key: key, Value: value}
		idx++
	}
	_, err := s.client.PutVespiaryEvents(s.ctx, &api.PutVespiaryEventRequest{
		Events: []*api.VespiaryAuditEvent{
			{Timestamp: time.Now().UnixNano(), Tenant: tenant, Kind: string(eventKind), Attributes: attributes},
		},
	})
	return err
}
func GRPCRecorder(remote *grpc.ClientConn, logger *zap.Logger) Recorder {
	client := api.NewVespiaryAuditRecorderClient(remote)
	return &grpcRecorder{client: client, ctx: context.Background(), logger: logger}
}
