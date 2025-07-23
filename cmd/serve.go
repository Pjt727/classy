/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/Pjt727/classy/server"
	"github.com/spf13/cobra"
)

// serveapiCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Runs the api service",
	Long:  `Runs the api service`,
	Run: func(cmd *cobra.Command, args []string) {
		api.Serve()
	},
}

func init() {
	appCmd.AddCommand(serveCmd)
}
