package main

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	consulapi "github.com/hashicorp/consul/api"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/cluster"
	"github.com/vx-labs/cluster/raft"
	"github.com/vx-labs/vespiary/vespiary"
	"github.com/vx-labs/vespiary/vespiary/fsm"
	"github.com/vx-labs/vespiary/vespiary/state"
	"github.com/vx-labs/wasp/v4/async"
	"go.etcd.io/etcd/etcdserver/api/snap"

	"github.com/vx-labs/wasp/v4/rpc"
	"go.uber.org/zap"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func localPrivateHost() string {
	// Maybe we are running on fly.io
	flyLocalAddr, err := net.ResolveIPAddr("ip", "fly-local-6pn")
	if err == nil {
		return flyLocalAddr.IP.String()
	}
	// last attempt: use the default outgoing iface.
	conn, err := net.Dial("udp", "example.net:80")
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
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
			healthServer.SetServingStatus("rpc", healthpb.HealthCheckResponse_NOT_SERVING)

			if healthPort := config.GetInt("health-port"); healthPort != 0 {
				addr := net.JoinHostPort("::", fmt.Sprintf("%d", healthPort))
				go func() {
					mux := http.NewServeMux()
					mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
						out, err := healthServer.Check(r.Context(), &healthpb.HealthCheckRequest{
							Service: "rpc",
						})
						if err != nil {
							w.WriteHeader(http.StatusInternalServerError)
							return
						}
						switch out.Status {
						case grpc_health_v1.HealthCheckResponse_SERVING:
							w.WriteHeader(http.StatusOK)
						case grpc_health_v1.HealthCheckResponse_NOT_SERVING:
							w.WriteHeader(http.StatusTooManyRequests)
						default:
							w.WriteHeader(http.StatusInternalServerError)
						}
						return
					})
					panic(http.ListenAndServe(addr, mux))
				}()
				vespiary.L(ctx).Info("started health server", zap.String("pprof_url", fmt.Sprintf("http://%s/", addr)))
			}

			stateStore := state.NewStateStore()
			healthServer.Resume()
			operations := async.NewOperations(ctx, vespiary.L(ctx))
			cancelCh := make(chan struct{})
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
			rpcDialer := rpc.GRPCDialer(rpc.ClientConfig{
				InsecureSkipVerify:          config.GetBool("insecure"),
				TLSCertificatePath:          config.GetString("rpc-tls-certificate-file"),
				TLSPrivateKeyPath:           config.GetString("rpc-tls-private-key-file"),
				TLSCertificateAuthorityPath: config.GetString("rpc-tls-certificate-authority-file"),
			})
			clusterListener, err := net.Listen("tcp", fmt.Sprintf("[::]:%d", config.GetInt("raft-port")))
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
			auditRecorder, err := getAuditRecorder(ctx, rpcDialer, config, vespiary.L(ctx))
			if err != nil {
				vespiary.L(ctx).Fatal("failed to create audit recorder", zap.Error(err))
			}

			var stateMachine *fsm.FSM

			raftConfig := cluster.RaftConfig{
				GetStateSnapshot: stateStore.Dump,
				CommitApplier: func(ctx context.Context, event raft.Commit) error {
					return stateMachine.Apply(event.Index, event.Payload)
				},
				SnapshotApplier: func(ctx context.Context, index uint64, snapshotter *snap.Snapshotter) error {
					snapshot, err := snapshotter.Load()
					if err != nil {
						vespiary.L(ctx).Error("failed to load snapshot", zap.Error(err))
						return err
					}
					return stateStore.Load(snapshot.Data)
				},
				ExpectedNodeCount: config.GetInt("raft-bootstrap-expect"),
				Network: cluster.NetworkConfig{
					AdvertizedHost: config.GetString("raft-advertized-address"),
					AdvertizedPort: config.GetInt("raft-advertized-port"),
					ListeningPort:  config.GetInt("raft-port"),
				},
			}
			clusterMultiNode := cluster.NewMultiNode(cluster.NodeConfig{
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
				RaftConfig: raftConfig,
			}, rpcDialer, server, vespiary.L(ctx))
			clusterNode := clusterMultiNode.Node("vespiary", raftConfig)
			stateMachine = fsm.NewFSM(id, stateStore, clusterNode, auditRecorder)

			operations.Run("cluster node", func(ctx context.Context) {
				clusterNode.Run(ctx)
			})

			vespiaryServer := vespiary.NewServer(stateMachine, stateStore)
			vespiaryServer.Serve(server)
			adminCertPool := x509.NewCertPool()
			if adminCA := config.GetString("rpc-tls-certificate-authority-file"); adminCA != "" {
				ca, err := ioutil.ReadFile(adminCA)
				if err != nil {
					panic(fmt.Errorf("could not read admin ca certificate: %s", err))
				}

				// Append the client certificates from the CA
				if ok := adminCertPool.AppendCertsFromPEM(ca); !ok {
					panic(errors.New("failed to append client certs to admin certificate pool"))
				}
			}

			waspAuthServer := vespiary.NewWaspAuthenticationServer(stateMachine, stateStore, adminCertPool)
			waspAuthServer.Serve(server)
			operations.Run("cluster listener", func(ctx context.Context) {
				err := server.Serve(clusterListener)
				if err != nil {
					panic(err)
				}
			})

			<-clusterNode.Ready()
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
			err = clusterNode.Shutdown()
			if err != nil {
				vespiary.L(ctx).Error("failed to leave cluster", zap.Error(err))
			} else {
				vespiary.L(ctx).Debug("cluster left")
			}
			clusterMultiNode.Shutdown()
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
			operations.Wait()
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
	cmd.Flags().Int("health-port", 0, "Start HTTP healthcheck server on this port.")
	cmd.Flags().Int("serf-port", 3799, "Membership (Serf) port.")
	cmd.Flags().Int("raft-port", 3899, "Clustering (Raft) port.")
	cmd.Flags().String("serf-advertized-address", defaultIP, "Advertize this adress to other gossip members.")
	cmd.Flags().String("raft-advertized-address", defaultIP, "Advertize this adress to other raft nodes.")
	cmd.Flags().Int("serf-advertized-port", 3799, "Advertize this port to other gossip members.")
	cmd.Flags().Int("raft-advertized-port", 3899, "Advertize this port to other raft nodes.")
	cmd.Flags().StringSliceP("join-node", "j", nil, "Join theses nodes to form a cluster.")
	cmd.Flags().StringP("data-dir", "d", "/tmp/vespiary", "vespiary persistent data location.")

	cmd.Flags().IntP("raft-bootstrap-expect", "n", 1, "vespiary will wait for this number of nodes to be available before bootstraping a cluster.")

	cmd.Flags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Server.")
	cmd.Flags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Server.")
	cmd.Flags().String("rpc-tls-private-key-file", "", "Private key used by RPC Server.")

	cmd.Flags().String("audit-recorder", "none", "Audit recorder used to record state updates. Set to \"none\" to disable audit.")
	cmd.Flags().String("audit-recorder-grpc-address", "", "GRPC Audit Recorder server address, when using \"grpc\" audit recorder.")

	cmd.AddCommand(TLSHelper(config))
	cmd.Execute()
}
