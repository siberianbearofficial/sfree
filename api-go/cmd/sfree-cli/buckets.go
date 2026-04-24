package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type bucketItem struct {
	ID        string `json:"id"`
	Key       string `json:"key"`
	AccessKey string `json:"access_key"`
	CreatedAt string `json:"created_at"`
}

type createBucketReq struct {
	Key       string   `json:"key"`
	SourceIDs []string `json:"source_ids"`
}

type createBucketResp struct {
	Key          string `json:"key"`
	AccessKey    string `json:"access_key"`
	AccessSecret string `json:"access_secret"`
	CreatedAt    string `json:"created_at"`
}

var bucketsCmd = &cobra.Command{
	Use:   "buckets",
	Short: "Manage buckets",
}

var bucketsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all buckets",
	RunE: func(cmd *cobra.Command, args []string) error {
		var buckets []bucketItem
		if err := apiGet("/api/v1/buckets", &buckets); err != nil {
			return err
		}
		if len(buckets) == 0 {
			fmt.Println("No buckets configured.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "ID\tKEY\tACCESS KEY\tCREATED"); err != nil {
			return err
		}
		for _, b := range buckets {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", b.ID, b.Key, b.AccessKey, b.CreatedAt); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

var (
	bucketCreateKey     string
	bucketCreateSources string
)

var bucketsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new bucket",
	Long: `Create a new bucket backed by the specified storage sources.
S3-compatible access credentials are generated automatically.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if bucketCreateKey == "" {
			return fmt.Errorf("--key is required")
		}
		if bucketCreateSources == "" {
			return fmt.Errorf("--sources is required (comma-separated source IDs)")
		}
		ids := strings.Split(bucketCreateSources, ",")
		var resp createBucketResp
		if err := apiPost("/api/v1/buckets", createBucketReq{
			Key:       bucketCreateKey,
			SourceIDs: ids,
		}, &resp); err != nil {
			return err
		}
		fmt.Println("Bucket created successfully.")
		fmt.Println()
		fmt.Printf("  Bucket Key:      %s\n", resp.Key)
		fmt.Printf("  Access Key:      %s\n", resp.AccessKey)
		fmt.Printf("  Access Secret:   %s\n", resp.AccessSecret)
		fmt.Printf("  Created:         %s\n", resp.CreatedAt)
		fmt.Println()
		fmt.Println("Use these credentials with any S3-compatible client:")
		fmt.Printf("  aws s3 ls s3://%s/ --endpoint-url %s\n", resp.Key, serverURL())
		return nil
	},
}

func init() {
	bucketsCreateCmd.Flags().StringVar(&bucketCreateKey, "key", "", "Bucket key (name)")
	bucketsCreateCmd.Flags().StringVar(&bucketCreateSources, "sources", "", "Comma-separated source IDs")
	bucketsCmd.AddCommand(bucketsListCmd, bucketsCreateCmd)
	rootCmd.AddCommand(bucketsCmd)
}
