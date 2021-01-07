package main

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	cluster "github.com/vx-labs/cluster/clusterpb"
	"github.com/vx-labs/wasp/v4/wasp/auth"
	"go.uber.org/zap"
)

func seedRand() {
	var b [8]byte
	_, err := crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package with cryptographically secure random number generator")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))
}
func main() {
	seedRand()
	config := viper.New()
	config.AddConfigPath(configDir())
	config.SetConfigType("yaml")
	config.SetConfigName("config")
	config.SetEnvPrefix("VESPIARYCTL")
	config.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	config.AutomaticEnv()
	ctx := context.Background()
	rootCmd := &cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			config.BindPFlags(cmd.Flags())
			config.BindPFlags(cmd.PersistentFlags())
			if err := config.ReadInConfig(); err != nil {
				if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
					log.Fatal(err)
				}
			}
		},
	}
	raft := &cobra.Command{
		Use: "raft",
	}
	raft.AddCommand(&cobra.Command{
		Use:     "topology",
		Aliases: []string{"members", "topo"},
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := cluster.NewMultiRaftClient(conn).GetTopology(ctx, &cluster.GetTopologyRequest{
				ClusterID: "vespiary",
			})
			if err != nil {
				l.Fatal("failed to get raft topology", zap.Error(err))
			}
			sort.SliceStable(out.Members, func(i, j int) bool {
				return out.Members[i].ID < out.Members[j].ID
			})
			table := getTable([]string{"ID", "Suffrage", "Address", "Progress", "Health", "Latency"}, cmd.OutOrStdout())
			for _, member := range out.GetMembers() {
				healthString := "healthy"
				if !member.IsAlive {
					healthString = "unhealthy"
				}
				suffrage := "learner"
				if member.IsLeader {
					suffrage = "leader"
				} else if member.IsVoter {
					suffrage = "voter"
				}
				table.Append([]string{
					fmt.Sprintf("%x", member.GetID()),
					suffrage,
					member.GetAddress(),
					fmt.Sprintf("%d/%d", member.GetApplied(), out.Committed),
					healthString,
					fmt.Sprintf("%dms", member.LatencyMs),
				})
			}
			table.Render()
		},
	})
	raft.AddCommand(&cobra.Command{
		Use:     "remove-member",
		Aliases: []string{"rm"},
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			for _, arg := range args {
				v, err := strconv.ParseUint(arg, 16, 64)
				if err != nil {
					l.Fatal("failed to parse member id", zap.Error(err))
				}
				_, err = cluster.NewMultiRaftClient(conn).RemoveMember(ctx, &cluster.RemoveMultiRaftMemberRequest{
					ID:        v,
					ClusterID: "vespiary",
					Force:     true,
				})
				if err != nil {
					l.Fatal("failed to get raft topology", zap.Error(err))
				}
			}
		},
	})

	hostname, _ := os.Hostname()

	authenticate := (&cobra.Command{
		Use:     "test-authentication",
		Aliases: []string{"test", "test-auth"},
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			resp, err := auth.NewAuthenticationClient(conn).AuthenticateMQTTClient(ctx, &auth.WaspAuthenticationRequest{
				MQTT: &auth.ApplicationContext{
					ClientID: []byte(config.GetString("client-id")),
					Username: []byte(config.GetString("username")),
					Password: []byte(config.GetString("password")),
				},
				Transport: &auth.TransportContext{
					Encrypted: config.GetBool("encrypted"),
				},
			})
			if err != nil {
				l.Fatal("authentication failed", zap.Error(err))
			}
			fmt.Printf("authentication succeeded: session_id=%s, tenant=%s\n", resp.ID, resp.MountPoint)
		},
	})
	authenticate.Flags().StringP("client-id", "i", "", "MQTT Client-ID")
	authenticate.Flags().StringP("username", "u", "", "MQTT Username")
	authenticate.Flags().StringP("password", "p", "", "MQTT Password")
	authenticate.Flags().BoolP("encrypted", "e", false, "Flag transport as Encrypted")

	rootCmd.AddCommand(authenticate)

	rootCmd.AddCommand(raft)
	rootCmd.AddCommand(Devices(ctx, config))
	rootCmd.AddCommand(Applications(ctx, config))
	rootCmd.AddCommand(ApplicationProfiles(ctx, config))
	rootCmd.AddCommand(Accounts(ctx, config))
	rootCmd.PersistentFlags().BoolP("insecure", "k", false, "Disable GRPC client-side TLS validation.")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Increase log verbosity.")
	rootCmd.PersistentFlags().BoolP("use-vault", "v", false, "Use Hashicorp Vault to generate GRPC Certificates.")
	rootCmd.PersistentFlags().String("vault-pki-path", "pki/issue/grpc", "Vault PKI certificate issuing path.")
	rootCmd.PersistentFlags().String("vault-pki-common-name", hostname, "Vault PKI certificate Common Name to submit.")
	rootCmd.PersistentFlags().BoolP("use-consul", "c", false, "Use Hashicorp Consul to find vespiary server.")
	rootCmd.PersistentFlags().String("consul-service-name", "vespiary", "Consul service name.")
	rootCmd.PersistentFlags().String("consul-service-tag", "rpc", "Consul service tag.")
	rootCmd.PersistentFlags().String("host", "127.0.0.1:1899", "remote GRPC endpoint")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-authority-file", "", "x509 certificate authority used by RPC Client.")
	rootCmd.PersistentFlags().String("rpc-tls-certificate-file", "", "x509 certificate used by RPC Client.")
	rootCmd.PersistentFlags().String("rpc-tls-private-key-file", "", "Private key used by RPC Client.")
	rootCmd.Execute()
}
