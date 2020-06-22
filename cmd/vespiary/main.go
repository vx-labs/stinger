package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary"
	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/vespiary/vespiary/async"
	"github.com/vx-labs/vespiary/vespiary/fsm"
	"github.com/vx-labs/vespiary/vespiary/rpc"
	"github.com/vx-labs/wasp/cluster"
	"github.com/vx-labs/wasp/cluster/raft"
	"go.uber.org/zap"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func localPrivateHost() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic(err)
	}

	for _, v := range ifaces {
		if v.Flags&net.FlagLoopback != net.FlagLoopback && v.Flags&net.FlagUp == net.FlagUp {
			h := v.HardwareAddr.String()
			if len(h) == 0 {
				continue
			} else {
				addresses, _ := v.Addrs()
				if len(addresses) > 0 {
					ip := addresses[0]
					if ipnet, ok := ip.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
						if ipnet.IP.To4() != nil {
							return ipnet.IP.String()
						}
					}
				}
			}
		}
	}
	panic("could not find a valid network interface")
}
func findPeers(name, tag string, minimumCount int) ([]string, error) {
	config := consulapi.DefaultConfig()
	config.HttpClient = http.DefaultClient
	client, err := consulapi.NewClient(config)
	if err != nil {
		return nil, err
	}
	var idx uint64
	for {
		services, meta, err := client.Catalog().Service(name, tag, &consulapi.QueryOptions{
			WaitIndex: idx,
			WaitTime:  10 * time.Second,
		})
		if err != nil {
			return nil, err
		}
		idx = meta.LastIndex
		if len(services) < minimumCount {
			continue
		}
		out := make([]string, len(services))
		for idx := range services {
			out[idx] = fmt.Sprintf("%s:%d", services[idx].ServiceAddress, services[idx].ServicePort)
		}
		return out, nil
	}
}

func main() {
	config := viper.New()
	config.SetEnvPrefix("vespiary")
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	config.AutomaticEnv()
	cmd := cobra.Command{
		Use: "vespiary",
		PreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlags(cmd.Flags())
			if !cmd.Flags().Changed("serf-advertized-port") {
				config.Set("serf-advertized-port", config.Get("serf-port"))
			}
			if !cmd.Flags().Changed("raft-advertized-port") {
				config.Set("raft-advertized-port", config.Get("raft-port"))
			}

		},
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithCancel(context.Background())
			ctx = vespiary.StoreLogger(ctx, getLogger(config))
			err := os.MkdirAll(config.GetString("data-dir"), 0700)
			if err != nil {
				vespiary.L(ctx).Fatal("failed to create data directory", zap.Error(err))
			}
			id, err := loadID(config.GetString("data-dir"))
			if err != nil {
				vespiary.L(ctx).Fatal("failed to get node ID", zap.Error(err))
			}
			ctx = vespiary.AddFields(ctx, zap.String("hex_node_id", fmt.Sprintf("%x", id)))
			if config.GetBool("pprof") {
				address := fmt.Sprintf("%s:%d", config.GetString("pprof-address"), config.GetInt("pprof-port"))
				go func() {
					mux := http.NewServeMux()
					mux.HandleFunc("/debug/pprof/", pprof.Index)
					mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
					mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
					mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
					mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
					panic(http.ListenAndServe(address, mux))
				}()
				vespiary.L(ctx).Info("started pprof", zap.String("pprof_url", fmt.Sprintf("http://%s/", address)))
			}
			healthServer := health.NewServer()
			stateStore := vespiary.NewStateStore()
			healthServer.Resume()
			wg := sync.WaitGroup{}
			cancelCh := make(chan struct{})
			commandsCh := make(chan raft.Command)
			if config.GetString("rpc-tls-certificate-file") == "" || config.GetString("rpc-tls-private-key-file") == "" {
				vespiary.L(ctx).Warn("TLS certificate or private key not provided. GRPC transport security will use a self-signed generated certificate.")
			}
			server := rpc.Server(rpc.ServerConfig{
				VerifyClientCert:            config.GetBool("mtls"),
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
				TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
				TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
			})
			healthpb.RegisterHealthServer(server, healthServer)
			api.RegisterNodeServer(server, rpc.NewNodeRPCServer(cancelCh))
			rpcDialer := rpc.GRPCDialer(rpc.ClientConfig{
				InsecureSkipVerify:          config.GetBool("insecure"),
				TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
				TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
			})
			clusterListener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", config.GetInt("raft-port")))
			if err != nil {
				vespiary.L(ctx).Fatal("cluster listener failed to start", zap.Error(err))
			}
			joinList := config.GetStringSlice("join-node")
			if config.GetBool("consul-join") {
				discoveryStarted := time.Now()
				consulJoinList, err := findPeers(
					config.GetString("consul-service-name"), config.GetString("consul-service-tag"),
					config.GetInt("raft-bootstrap-expect"))
				if err != nil {
					vespiary.L(ctx).Fatal("failed to find other peers on Consul", zap.Error(err))
				}
				vespiary.L(ctx).Info("discovered nodes using Consul",
					zap.Duration("consul_discovery_duration", time.Since(discoveryStarted)), zap.Int("node_count", len(consulJoinList)))
				joinList = append(joinList, consulJoinList...)
			}

			clusterNode := cluster.NewNode(cluster.NodeConfig{
				ID:            id,
				ServiceName:   "vespiary",
				DataDirectory: config.GetString("data-dir"),
				GossipConfig: cluster.GossipConfig{
					JoinList: joinList,
					Network: cluster.NetworkConfig{
						AdvertizedHost: config.GetString("serf-advertized-address"),
						AdvertizedPort: config.GetInt("serf-advertized-port"),
						ListeningPort:  config.GetInt("serf-port"),
					},
				},
				RaftConfig: cluster.RaftConfig{
					GetStateSnapshot:  stateStore.Dump,
					ExpectedNodeCount: config.GetInt("raft-bootstrap-expect"),
					Network: cluster.NetworkConfig{
						AdvertizedHost: config.GetString("raft-advertized-address"),
						AdvertizedPort: config.GetInt("raft-advertized-port"),
						ListeningPort:  config.GetInt("raft-port"),
					},
				},
			}, rpcDialer, server, vespiary.L(ctx))

			async.Run(ctx, &wg, func(ctx context.Context) {
				defer vespiary.L(ctx).Debug("cluster node stopped")
				clusterNode.Run(ctx)
			})

			stateMachine := fsm.NewFSM(id, stateStore, commandsCh)
			vespiaryServer := vespiary.NewServer(stateMachine, stateStore)
			vespiaryServer.Serve(server)
			waspAuthServer := vespiary.NewWaspAuthenticationServer(stateMachine, stateStore)
			waspAuthServer.Serve(server)
			async.Run(ctx, &wg, func(ctx context.Context) {
				defer vespiary.L(ctx).Info("cluster listener stopped")

				err := server.Serve(clusterListener)
				if err != nil {
					vespiary.L(ctx).Fatal("cluster listener crashed", zap.Error(err))
				}
			})

			snapshotter := <-clusterNode.Snapshotter()
			async.Run(ctx, &wg, func(ctx context.Context) {
				defer vespiary.L(ctx).Info("command publisher stopped")
				for {
					select {
					case <-ctx.Done():
						return
					case event := <-commandsCh:
						err := clusterNode.Apply(event.Ctx, event.Payload)
						select {
						case <-ctx.Done():
						case <-event.Ctx.Done():
						case event.ErrCh <- err:
						}
						close(event.ErrCh)
					}
				}
			})
			async.Run(ctx, &wg, func(ctx context.Context) {
				defer vespiary.L(ctx).Info("command processor stopped")
				for {
					select {
					case <-ctx.Done():
						return
					case event := <-clusterNode.Commits():
						if event.Payload == nil {
							snapshot, err := snapshotter.Load()
							if err != nil {
								vespiary.L(ctx).Error("failed to load snapshot", zap.Error(err))
								break
							}
							err = stateStore.Load(snapshot.Data)
							if err != nil {
								vespiary.L(ctx).Error("failed to load snapshot in store", zap.Error(err))
							}
						} else {
							stateMachine.Apply(event.Index, event.Payload)
						}
					}
				}
			})
			<-clusterNode.Ready()

			healthServer.SetServingStatus("mqtt", healthpb.HealthCheckResponse_SERVING)
			healthServer.SetServingStatus("node", healthpb.HealthCheckResponse_SERVING)
			healthServer.SetServingStatus("rpc", healthpb.HealthCheckResponse_SERVING)
			sigc := make(chan os.Signal, 1)
			signal.Notify(sigc,
				syscall.SIGINT,
				syscall.SIGTERM,
				syscall.SIGQUIT)
			select {
			case <-sigc:
			case <-cancelCh:
			}
			vespiary.L(ctx).Info("vespiary shutdown initiated")
			err = stateMachine.Shutdown(ctx)
			if err != nil {
				vespiary.L(ctx).Error("failed to stop state machine", zap.Error(err))
			} else {
				vespiary.L(ctx).Debug("state machine stopped")
			}
			err = clusterNode.Shutdown()
			if err != nil {
				vespiary.L(ctx).Error("failed to leave cluster", zap.Error(err))
			} else {
				vespiary.L(ctx).Debug("cluster left")
			}
			healthServer.Shutdown()
			vespiary.L(ctx).Debug("health server stopped")
			go func() {
				<-time.After(1 * time.Second)
				server.Stop()
			}()
			server.GracefulStop()
			vespiary.L(ctx).Debug("rpc server stopped")
			clusterListener.Close()
			vespiary.L(ctx).Debug("rpc listener stopped")
			cancel()
			wg.Wait()
			vespiary.L(ctx).Debug("asynchronous operations stopped")
			vespiary.L(ctx).Info("vespiary successfully stopped")
		},
	}
	defaultIP := localPrivateHost()
	cmd.Flags().Bool("pprof", false, "Start pprof endpoint.")
	cmd.Flags().Int("pprof-port", 8080, "Profiling (pprof) port.")
	cmd.Flags().String("pprof-address", "127.0.0.1", "Profiling (pprof) port.")
	cmd.Flags().Bool("debug", false, "Use a fancy logger and increase logging level.")
	cmd.Flags().Bool("mtls", false, "Enforce GRPC service-side TLS certificates validation for client connections.")
	cmd.Flags().Bool("insecure", false, "Disable GRPC client-side TLS validation.")
	cmd.Flags().Bool("consul-join", false, "Use Hashicorp Consul to find other gossip members. vespiary won't handle service registration in Consul, you must do it before running vespiary.")
	cmd.Flags().String("consul-service-name", "vespiary", "Consul auto-join service name.")
	cmd.Flags().String("consul-service-tag", "gossip", "Consul auto-join service tag.")

	cmd.Flags().Int("metrics-port", 0, "Start Prometheus HTTP metrics server on this port.")
	cmd.Flags().Int("serf-port", 3799, "Membership (Serf) port.")
	cmd.Flags().Int("raft-port", 3899, "Clustering (Raft) port.")
	cmd.Flags().String("serf-advertized-address", defaultIP, "Advertize this adress to other gossip members.")
	cmd.Flags().String("raft-advertized-address", defaultIP, "Advertize this adress to other raft nodes.")
	cmd.Flags().Int("serf-advertized-port", 2799, "Advertize this port to other gossip members.")
	cmd.Flags().Int("raft-advertized-port", 2899, "Advertize this port to other raft nodes.")
	cmd.Flags().StringSliceP("join-node", "j", nil, "Join theses nodes to form a cluster.")
	cmd.Flags().StringP("data-dir", "d", "/tmp/vespiary", "vespiary persistent data location.")

	cmd.Flags().IntP("raft-bootstrap-expect", "n", 1, "vespiary will wait for this number of nodes to be available before bootstraping a cluster.")

	cmd.Flags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Server.")
	cmd.Flags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Server.")
	cmd.Flags().String("rpc-tls-private-key-file", "", "Private key used by RPC Server.")

	cmd.AddCommand(TLSHelper(config))
	cmd.Execute()
}
