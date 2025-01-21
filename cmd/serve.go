/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/Pjt727/classy/api"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Runs the api service",
	Long:  `Runs the api service`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetLevel(log.TraceLevel)
		api.Serve()
	},
}

func init() {
	appCmd.AddCommand(serveCmd)
}
