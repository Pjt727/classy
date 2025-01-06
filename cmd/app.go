/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// appCmd represents the app command
var appCmd = &cobra.Command{
	Use:   "app",
	Short: "used to run the classy service",
	Long: `The classy service is a text and json server which serves
an api for class information form a variety of schools (this command is not ran directly)`,
}

func init() {
	rootCmd.AddCommand(appCmd)
}
