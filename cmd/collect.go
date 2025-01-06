/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// collectCmd represents the collect command
var collectCmd = &cobra.Command{
	Use:   "collect",
	Short: "collect school information",
	Long: `This is the root command to instruct classy on what school 
information to collect (this command is not ran directly)`,
}

func init() {
	rootCmd.AddCommand(collectCmd)
}
