package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/api"
	"go.uber.org/zap"
)

const applicationTemplate = `{{ .ID }}
  Name: {{ .Name }}
  Principals: {{ .Principals }}
  Device Usernames: {{ .DeviceUsernames }}
`

func Applications(ctx context.Context, config *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "applications",
		Aliases: []string{"application", "app"},
	}
	create := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).CreateApplication(ctx, &api.CreateApplicationRequest{
				Name:      config.GetString("name"),
				AccountID: config.GetString("account-id"),
			})
			if err != nil {
				l.Fatal("failed to create application", zap.Error(err))
			}
			fmt.Println(out.ID)
		},
	}
	create.Flags().StringP("name", "n", "", "New application friendly name")
	create.Flags().StringP("account-id", "i", "", "New application account id")

	cmd.AddCommand(create)

	list := (&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).ListApplications(ctx, &api.ListApplicationsRequest{})
			if err != nil {
				l.Fatal("failed to list applications", zap.Error(err))
			}
			table := getTable([]string{"ID", "AccountID", "Name"}, cmd.OutOrStdout())
			for _, application := range out.Applications {
				table.Append([]string{application.ID, application.AccountID, application.Name})
			}
			table.Render()
		},
	})

	cmd.AddCommand(list)

	delete := (&cobra.Command{
		Use:  "delete",
		Args: cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			for _, id := range args {
				_, err := api.NewVespiaryClient(conn).DeleteApplication(ctx, &api.DeleteApplicationRequest{
					ID: id,
				})
				if err != nil {
					l.Fatal("failed to delete application", zap.Error(err))
				}
			}
		},
	})
	cmd.AddCommand(delete)

	return cmd
}
