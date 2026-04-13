// Package main implements the sfree CLI tool.
package main

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	flagServer   string
	flagUser     string
	flagPassword string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "sfree",
	Short: "SFree CLI — manage storage sources, buckets, and files",
	Long: `SFree CLI lets you interact with a SFree server from the terminal.

Create storage sources, manage buckets, upload and download files,
and generate S3-compatible access keys.

Configuration via flags or environment variables:
  --server / SFREE_SERVER   API base URL (default http://localhost:8080)
  --user / SFREE_USER       Username for Basic Auth
  --password / SFREE_PASSWORD  Password for Basic Auth`,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagServer, "server", "", "SFree API base URL (env: SFREE_SERVER)")
	rootCmd.PersistentFlags().StringVar(&flagUser, "user", "", "Username (env: SFREE_USER)")
	rootCmd.PersistentFlags().StringVar(&flagPassword, "password", "", "Password (env: SFREE_PASSWORD)")
}

func serverURL() string {
	if flagServer != "" {
		return flagServer
	}
	if v := os.Getenv("SFREE_SERVER"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

func authHeader() (string, error) {
	user := flagUser
	if user == "" {
		user = os.Getenv("SFREE_USER")
	}
	pass := flagPassword
	if pass == "" {
		pass = os.Getenv("SFREE_PASSWORD")
	}
	if user == "" || pass == "" {
		return "", fmt.Errorf("username and password required (use --user/--password or SFREE_USER/SFREE_PASSWORD)")
	}
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass)), nil
}
