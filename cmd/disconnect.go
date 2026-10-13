/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeaforx/wiregam/internal/config"
	"github.com/zeaforx/wiregam/internal/disconnect"
)

var providerID string

// disconnectCmd represents the disconnect command
var disconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect vpn from Cloud VM",
	Long:  `This command will disconnect the vpn from the cloud VM. The VM would still be running but the vpn connection will be terminated. You can reconnect to the VM using the connect command.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		providerId, err := config.LoadDefaultProvider()
		if err != nil {
			fmt.Printf("Default provider has not been configured, try wiregam auth to set a default provider, or wiregam switch (not yet implemented though) switch to an already authenticated cloud provider")
			return err
		}
		providerID = providerId
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return disconnect.DefaultService().Disconnect(cmd.Context(), providerID)
	},
}

func init() {
	rootCmd.AddCommand(disconnectCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// disconnectCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// disconnectCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
