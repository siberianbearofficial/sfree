package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage S3-compatible access keys",
}

var (
	keysCreateBucket  string
	keysCreateSources string
)

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create S3 access keys by creating a new bucket",
	Long: `Create a new bucket and display its S3-compatible access credentials.
These credentials can be used with any S3-compatible client (aws-cli, mc, etc).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if keysCreateBucket == "" {
			return fmt.Errorf("--bucket is required")
		}
		if keysCreateSources == "" {
			return fmt.Errorf("--sources is required (comma-separated source IDs)")
		}
		ids := strings.Split(keysCreateSources, ",")
		var resp createBucketResp
		if err := apiPost("/api/v1/buckets", createBucketReq{
			Key:       keysCreateBucket,
			SourceIDs: ids,
		}, &resp); err != nil {
			return err
		}
		fmt.Println("S3 credentials created:")
		fmt.Println()
		fmt.Printf("  Bucket:          %s\n", resp.Key)
		fmt.Printf("  Access Key ID:   %s\n", resp.AccessKey)
		fmt.Printf("  Secret Key:      %s\n", resp.AccessSecret)
		fmt.Println()
		fmt.Println("Example usage:")
		fmt.Printf("  export AWS_ACCESS_KEY_ID=%s\n", resp.AccessKey)
		fmt.Printf("  export AWS_SECRET_ACCESS_KEY=%s\n", resp.AccessSecret)
		fmt.Printf("  aws s3 ls s3://%s/ --endpoint-url %s/api/s3\n", resp.Key, serverURL())
		return nil
	},
}

func init() {
	keysCreateCmd.Flags().StringVar(&keysCreateBucket, "bucket", "", "Bucket key (name) to create")
	keysCreateCmd.Flags().StringVar(&keysCreateSources, "sources", "", "Comma-separated source IDs")
	keysCmd.AddCommand(keysCreateCmd)
	rootCmd.AddCommand(keysCmd)
}
