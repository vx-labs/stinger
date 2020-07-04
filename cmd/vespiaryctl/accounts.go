package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/api"
	"go.uber.org/zap"
)

const accountTemplate = `{{ .ID }}
  Name: {{ .Name }}
  Principals: {{ .Principals }}
  Device Usernames: {{ .DeviceUsernames }}
`

func Accounts(ctx context.Context, config *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "accounts",
		Aliases: []string{"account"},
	}
	create := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).CreateAccount(ctx, &api.CreateAccountRequest{
				Name:            config.GetString("name"),
				Principals:      config.GetStringSlice("principals"),
				DeviceUsernames: config.GetStringSlice("device-usernames"),
			})
			if err != nil {
				l.Fatal("failed to create account", zap.Error(err))
			}
			fmt.Println(out.ID)
		},
	}
	create.Flags().StringP("name", "n", "", "New account friendly name")
	create.Flags().StringSliceP("principals", "p", nil, "New account principals")
	create.Flags().StringSliceP("device-usernames", "u", nil, "New account device usernames")

	cmd.AddCommand(create)

	list := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).ListAccounts(ctx, &api.ListAccountsRequest{})
			if err != nil {
				l.Fatal("failed to list accounts", zap.Error(err))
			}
			table := getTable([]string{"ID", "Name", "Principals", "Usernames"}, cmd.OutOrStdout())
			for _, account := range out.Accounts {
				table.Append([]string{
					account.ID, account.Name,
					strings.Join(account.Principals, ", "),
					strings.Join(account.DeviceUsernames, ", ")})
			}
			table.Render()

		},
	}
	cmd.AddCommand(list)
	byPrincipal := &cobra.Command{
		Use: "by-principal",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			out, err := api.NewVespiaryClient(conn).GetAccountByPrincipal(ctx, &api.GetAccountByPrincipalRequest{Principal: args[0]})
			if err != nil {
				l.Fatal("failed to list accounts", zap.Error(err))
			}
			account := out.Account
			table := getTable([]string{"ID", "Name", "Principals", "Usernames"}, cmd.OutOrStdout())
			table.Append([]string{
				account.ID, account.Name,
				strings.Join(account.Principals, ", "),
				strings.Join(account.DeviceUsernames, ", ")})

			table.Render()

		},
	}
	cmd.AddCommand(byPrincipal)

	delete := (&cobra.Command{
		Use: "delete",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).DeleteAccount(ctx, &api.DeleteAccountRequest{
				ID: config.GetString("id"),
			})
			if err != nil {
				l.Fatal("failed to delete account", zap.Error(err))
			}
		},
	})
	delete.Flags().StringP("id", "i", "", "Account ID")
	cmd.AddCommand(delete)

	return cmd
}
