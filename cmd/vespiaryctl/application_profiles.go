package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/api"
	"go.uber.org/zap"
)

func ApplicationProfiles(ctx context.Context, config *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "application-profiles",
		Aliases: []string{"profiles"},
	}
	create := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).CreateApplicationProfile(ctx, &api.CreateApplicationProfileRequest{
				Name:          config.GetString("name"),
				ApplicationID: config.GetString("application-id"),
				AccountID:     config.GetString("account-id"),
				Password:      config.GetString("password"),
			})
			if err != nil {
				l.Fatal("failed to create applicationProfile", zap.Error(err))
			}
			fmt.Println(out.ID)
		},
	}
	create.Flags().StringP("name", "n", "", "New applicationProfile friendly name")
	create.Flags().StringP("application-id", "a", "", "New application-profile application id")
	create.Flags().StringP("account-id", "i", "", "New application-profile account id")
	create.Flags().StringP("password", "p", "", "application-profile connection password")

	cmd.AddCommand(create)

	list := (&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).ListApplicationProfiles(ctx, &api.ListApplicationProfilesRequest{})
			if err != nil {
				l.Fatal("failed to list application profiles", zap.Error(err))
			}
			table := getTable([]string{"ID", "ApplicationID", "AccountID", "Name"}, cmd.OutOrStdout())
			for _, applicationProfile := range out.ApplicationProfiles {
				table.Append([]string{applicationProfile.ID, applicationProfile.ApplicationID, applicationProfile.AccountID, applicationProfile.Name})
			}
			table.Render()
		},
	})

	cmd.AddCommand(list)

	delete := (&cobra.Command{
		Use:     "delete",
		Aliases: []string{"rm"},
		Args:    cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			for _, id := range args {
				_, err := api.NewVespiaryClient(conn).DeleteApplicationProfile(ctx, &api.DeleteApplicationProfileRequest{
					ID: id,
				})
				if err != nil {
					l.Fatal("failed to delete application profile", zap.Error(err))
				}
			}
		},
	})
	cmd.AddCommand(delete)

	return cmd
}
