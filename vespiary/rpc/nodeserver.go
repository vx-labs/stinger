package rpc

import (
	"context"

	"github.com/vx-labs/vespiary/vespiary/api"
)

type NodeRPCServer struct {
	cancel chan<- struct{}
}

func NewNodeRPCServer(cancelCh chan<- struct{}) *NodeRPCServer {
	return &NodeRPCServer{cancel: cancelCh}
}

func (n *NodeRPCServer) Shutdown(ctx context.Context, _ *api.ShutdownRequest) (*api.ShutdownResponse, error) {
	n.cancel <- struct{}{}
	return &api.ShutdownResponse{}, nil
}
