package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"syscall"

	"github.com/Pjt727/classy/data"
	"github.com/Pjt727/classy/data/db"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

var (
	usernameFlag string
	passwordFlag string
)

// serveapiCmd represents the serve command
var addUser = &cobra.Command{
	Use:   "adduser",
	Short: "add a management user",
	Long:  `defaults to interactive but can add username and password with flags`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		dbPool, err := data.NewPool(ctx, false)
		if err != nil {
			fmt.Printf("Could not connect to the database %v", err)
			os.Exit(1)
		}
		q := db.New(dbPool)

		username := usernameFlag
		password := passwordFlag

		if username == "" {
			for {
				fmt.Print("Enter username: ")
				if _, err := fmt.Scanln(&username); err != nil {
					slog.Error("Failed to read username", "error", err)
					os.Exit(1)
				}
				username = strings.TrimSpace(username)
				if username == "" {
					fmt.Println("Username cannot be empty. Please try again.")
				} else {
					break
				}
			}
		}

		if password == "" {
			for {
				fmt.Print("Enter password: ")
				bytePassword, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println() // New line after password input
				if err != nil {
					slog.Error("Failed to read password", "error", err)
					os.Exit(1)
				}
				password = string(bytePassword)
				if password == "" {
					fmt.Println("Password cannot be empty. Please try again.")
					continue
				}

				fmt.Print("Confirm password: ")
				byteConfirmPassword, err := term.ReadPassword(int(syscall.Stdin))
				fmt.Println() // New line after confirmation input
				if err != nil {
					slog.Error("Failed to read password confirmation", "error", err)
					os.Exit(1)
				}
				confirmPassword := string(byteConfirmPassword)

				if password != confirmPassword {
					fmt.Println("Passwords do not match. Please try again.")
					password = ""
				} else {
					break
				}
			}
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			fmt.Printf("Could not encrypt password %v", err)
			os.Exit(1)
		}

		err = q.AuthInsertUser(ctx, db.AuthInsertUserParams{
			Username:          username,
			EncryptedPassword: string(hash),
		})
		if err != nil {
			fmt.Printf("Could not add managment user %v", err)
			os.Exit(1)
		}
		fmt.Printf("Added %s to the database", username)
	},
}

func init() {
	appCmd.AddCommand(addUser)
	addUser.Flags().StringVarP(&usernameFlag, "username", "u", "", "Username for the new management user")
	addUser.Flags().StringVarP(&passwordFlag, "password", "p", "", "Password for the new management user")
}
