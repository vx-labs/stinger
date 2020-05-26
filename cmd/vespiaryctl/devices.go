package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/api"
	"go.uber.org/zap"
)

const deviceTemplate = `{{ .ID }}
  Name: {{ .Name }}
  Owner: {{ .Owner }}
`

type record struct {
	Timestamp int64
	Topic     string
	Payload   string
}

func Devices(ctx context.Context, config *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use: "devices",
	}
	create := &cobra.Command{
		Use: "create",
		Run: func(cmd *cobra.Command, _ []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).CreateDevice(ctx, &api.CreateDeviceRequest{
				Name:     config.GetString("name"),
				Active:   config.GetBool("active"),
				Owner:    config.GetString("owner"),
				Password: config.GetString("password"),
			})
			if err != nil {
				l.Fatal("failed to create device", zap.Error(err))
			}
		},
	}
	create.Flags().StringP("name", "n", "", "New device friendly name")
	create.Flags().StringP("owner", "o", "", "New device Owner")
	create.Flags().Bool("active", true, "Is this new device active?")
	create.Flags().String("password", "", "New device password")

	cmd.AddCommand(create)

	get := (&cobra.Command{
		Use: "get",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			device, err := api.NewVespiaryClient(conn).GetDevice(ctx, &api.GetDeviceRequest{
				ID:    config.GetString("id"),
				Owner: config.GetString("owner"),
			})
			if err != nil {
				l.Fatal("failed to get device", zap.Error(err))
			}

			tpl := ParseTemplate(config.GetString("format"))
			tpl.Execute(cmd.OutOrStdout(), device)
		},
	})
	get.Flags().StringP("id", "i", "", "Device ID")
	get.Flags().StringP("owner", "o", "", "Device Owner")
	get.Flags().String("format", deviceTemplate, "Format output using Golang template format.")
	cmd.AddCommand(get)

	enable := (&cobra.Command{
		Use: "enable",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).EnableDevice(ctx, &api.EnableDeviceRequest{
				ID:    config.GetString("id"),
				Owner: config.GetString("owner"),
			})
			if err != nil {
				l.Fatal("failed to enable device", zap.Error(err))
			}
		},
	})
	enable.Flags().StringP("id", "i", "", "Device ID")
	enable.Flags().StringP("owner", "o", "", "Device Owner")
	cmd.AddCommand(enable)

	disable := (&cobra.Command{
		Use: "disable",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).DisableDevice(ctx, &api.DisableDeviceRequest{
				ID:    config.GetString("id"),
				Owner: config.GetString("owner"),
			})
			if err != nil {
				l.Fatal("failed to disable device", zap.Error(err))
			}
		},
	})
	disable.Flags().StringP("id", "i", "", "Device ID")
	disable.Flags().StringP("owner", "o", "", "Device Owner")
	cmd.AddCommand(disable)

	delete := (&cobra.Command{
		Use: "delete",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).DeleteDevice(ctx, &api.DeleteDeviceRequest{
				ID:    config.GetString("id"),
				Owner: config.GetString("owner"),
			})
			if err != nil {
				l.Fatal("failed to disable device", zap.Error(err))
			}
		},
	})
	delete.Flags().StringP("id", "i", "", "Device ID")
	delete.Flags().StringP("owner", "o", "", "Device Owner")
	cmd.AddCommand(delete)

	list := (&cobra.Command{
		Use: "list",
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			resp, err := api.NewVespiaryClient(conn).ListDevices(ctx, &api.ListDevicesRequest{
				Owner: config.GetString("owner"),
			})
			if err != nil {
				l.Fatal("failed to list devices", zap.Error(err))
			}
			table := getTable([]string{"ID", "Owner", "Name", "Active", "Password"}, cmd.OutOrStdout())
			for _, device := range resp.Devices {
				table.Append([]string{device.ID, device.Owner, device.Name, fmt.Sprintf("%v", device.Active), device.Password})
			}
			table.Render()
		},
	})
	list.Flags().StringP("owner", "o", "", "Device Owner")

	cmd.AddCommand(list)

	return cmd
}
