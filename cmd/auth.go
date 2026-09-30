/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zeaforx/wiregam/internal/auth"
	"github.com/zeaforx/wiregam/internal/config"
	"github.com/zeaforx/wiregam/internal/providers"
)

var provider string

var setDefault bool

var authService = auth.DefaultService()

// authCmd represents the auth command
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		selectedProvider, found := providers.Find(provider)
		if !found {
			return fmt.Errorf("provider %q does not exist", provider)
		}

		if !selectedProvider.IsAvailable {
			return fmt.Errorf("provider %q is not available yet", provider)
		}

		return nil
	},

	RunE: func(cmd *cobra.Command, args []string) error {
		if err := authService.Authenticate(context.Background(), provider); err != nil {
			return err
		}

		if setDefault {
			if err := config.StoreDefaultProvider(provider); err != nil {
				return err
			}
			fmt.Println("Default provider stored successfully!")
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(authCmd)

	authCmd.Flags().StringVar(&provider, "provider", "", "Choose cloud provider for vpn")
	if err := authCmd.MarkFlagRequired("provider"); err != nil {
		panic(err)
	}
	authCmd.Flags().BoolVar(&setDefault, "setDefault", true, "make this provider the default for future commands")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// authCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// authCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
