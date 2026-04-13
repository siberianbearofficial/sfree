package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type sourceItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}

var sourcesCmd = &cobra.Command{
	Use:   "sources",
	Short: "Manage storage sources",
}

var sourcesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured storage sources",
	RunE: func(cmd *cobra.Command, args []string) error {
		var sources []sourceItem
		if err := apiGet("/api/v1/sources", &sources); err != nil {
			return err
		}
		if len(sources) == 0 {
			fmt.Println("No sources configured.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tTYPE\tCREATED")
		for _, s := range sources {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.ID, s.Name, s.Type, s.CreatedAt)
		}
		return w.Flush()
	},
}

func init() {
	sourcesCmd.AddCommand(sourcesListCmd)
	rootCmd.AddCommand(sourcesCmd)
}
