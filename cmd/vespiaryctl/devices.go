package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/vx-labs/vespiary/vespiary/api"
	"github.com/vx-labs/wasp/wasp/auth"
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
		Use:     "delete",
		Aliases: []string{"rm"},
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
	changePassword := (&cobra.Command{
		Use:     "change-password",
		Aliases: []string{"passwd"},
		Run: func(cmd *cobra.Command, args []string) {
			conn, l := mustDial(ctx, cmd, config)
			_, err := api.NewVespiaryClient(conn).ChangeDevicePassword(ctx, &api.ChangeDevicePasswordRequest{
				ID:          config.GetString("id"),
				Owner:       config.GetString("owner"),
				NewPassword: config.GetString("password"),
			})
			if err != nil {
				l.Fatal("failed to change device password", zap.Error(err))
			}
		},
	})
	changePassword.Flags().StringP("id", "i", "", "Device ID")
	changePassword.Flags().StringP("owner", "o", "", "Device Owner")
	changePassword.Flags().StringP("password", "p", "", "Device new password")
	cmd.AddCommand(changePassword)

	list := (&cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
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
	authenticate := (&cobra.Command{
		Use:     "authenticate",
		Aliases: []string{"auth"},
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
			fmt.Printf("authentication succeed: session_id=%s, tenant=%s\n", resp.ID, resp.MountPoint)
		},
	})
	authenticate.Flags().StringP("client-id", "i", "", "MQTT Client-ID")
	authenticate.Flags().StringP("username", "u", "", "MQTT Username")
	authenticate.Flags().StringP("password", "p", "", "MQTT Password")
	authenticate.Flags().BoolP("encrypted", "e", false, "Flag transport as Encrypted")

	cmd.AddCommand(authenticate)

	return cmd
}
