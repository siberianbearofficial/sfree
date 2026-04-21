package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type fileItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

type uploadResp struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

var uploadCmd = &cobra.Command{
	Use:   "upload <bucket-id> <file-path>",
	Short: "Upload a file to a bucket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketID := args[0]
		filePath := args[1]
		var resp uploadResp
		if err := apiUpload(fmt.Sprintf("/api/v1/buckets/%s/upload", bucketID), filePath, &resp); err != nil {
			return err
		}
		fmt.Printf("Uploaded %s\n", resp.Name)
		fmt.Printf("  File ID: %s\n", resp.ID)
		return nil
	},
}

var downloadCmd = &cobra.Command{
	Use:   "download <bucket-id> <file-id> [output-path]",
	Short: "Download a file from a bucket",
	Args:  cobra.RangeArgs(2, 3),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketID := args[0]
		fileID := args[1]
		output := fileID
		if len(args) == 3 {
			output = args[2]
		}
		path := fmt.Sprintf("/api/v1/buckets/%s/files/%s/download", bucketID, fileID)
		if err := apiDownload(path, output); err != nil {
			return err
		}
		fmt.Printf("Downloaded to %s\n", output)
		return nil
	},
}

var filesCmd = &cobra.Command{
	Use:   "files",
	Short: "Manage files in buckets",
}

var filesListCmd = &cobra.Command{
	Use:   "list <bucket-id>",
	Short: "List files in a bucket",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketID := args[0]
		var files []fileItem
		if err := apiGet(fmt.Sprintf("/api/v1/buckets/%s/files", bucketID), &files); err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Println("No files in this bucket.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(w, "ID\tNAME\tSIZE\tCREATED"); err != nil {
			return err
		}
		for _, f := range files {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.ID, f.Name, humanSize(f.Size), f.CreatedAt); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

var filesDeleteCmd = &cobra.Command{
	Use:   "delete <bucket-id> <file-id>",
	Short: "Delete a file from a bucket",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		bucketID := args[0]
		fileID := args[1]
		if err := apiDelete(fmt.Sprintf("/api/v1/buckets/%s/files/%s", bucketID, fileID)); err != nil {
			return err
		}
		fmt.Println("File deleted.")
		return nil
	},
}

func init() {
	filesCmd.AddCommand(filesListCmd, filesDeleteCmd)
	rootCmd.AddCommand(uploadCmd, downloadCmd, filesCmd)
}

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func apiDelete(path string) error {
	auth, err := authHeader()
	if err != nil {
		return err
	}
	url := serverURL() + path
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", auth)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
