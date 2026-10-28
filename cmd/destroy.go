/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeaforx/wiregam/internal/destroy"
	"github.com/zeaforx/wiregam/internal/providers"
)

var destroyProvider string

var destroyService = destroy.DefaultService()

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Terminate WireGam cloud resources",
	Long: `Find EC2 instances created by WireGam in a selected AWS region, show the
matching instance IDs, and ask for confirmation before terminating them.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		selectedProvider, found := providers.Find(destroyProvider)
		if !found {
			return fmt.Errorf("provider %q does not exist", destroyProvider)
		}

		if !selectedProvider.IsAvailable {
			return fmt.Errorf("provider %q is not available yet", destroyProvider)
		}

		return destroyService.Destroy(cmd.Context(), destroyProvider)
	},
}

func init() {
	rootCmd.AddCommand(destroyCmd)

	destroyCmd.Flags().StringVar(&destroyProvider, "provider", "", "Choose cloud provider for VPN")
	if err := destroyCmd.MarkFlagRequired("provider"); err != nil {
		panic(err)
	}
}
