package meeting

import (
	"encoding/json"
	"os"
	"path/filepath"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	middleWare "tmeet/internal/cmdutil/middleware"
	"tmeet/internal/exception"
	"tmeet/internal/output"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// AttachmentDownloadOptions holds the options for downloading a meeting
// attachment to local disk.
type AttachmentDownloadOptions struct {
	tmeet     *internal.Tmeet
	MeetingID string
	FileID    string
	OutputDir string // Target directory; defaults to CWD. File name always comes from the server response.
	Overwrite bool   // Allow overwriting an existing target file.
}

// newAttachmentDownloadCmd downloads a meeting attachment to local disk by
// fetching a signed URL from the backend and streaming the bytes atomically
// via utils.DownloadFromCOS.
func newAttachmentDownloadCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &AttachmentDownloadOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "attachment-download",
		Short: "download a meeting attachment to local disk",
		RunE: middleWare.Chain(
			opts.Run,
			middleWare.WithApiCmd(cmdutil.StaticApiCmd(cmdutil.ApiCmdMeetingAttachmentDownload)),
		),
	}

	cmd.Flags().StringVar(&opts.MeetingID, "meeting-id", "", "meeting ID (required)")
	cmd.Flags().StringVar(&opts.FileID, "file-id", "", "attachment file ID (required)")
	cmd.Flags().StringVar(&opts.OutputDir, "output-dir", "",
		"target local directory; defaults to the current working directory. The file name is always taken from the server response.")
	cmd.Flags().BoolVar(&opts.Overwrite, "overwrite", false,
		"overwrite the target file if it already exists")

	_ = cmd.MarkFlagRequired("meeting-id")
	_ = cmd.MarkFlagRequired("file-id")
	return cmd
}

// Run fetches the signed URL, resolves the target directory (creating it
// when missing), and streams the attachment to <dir>/<server_file_name>.
func (o *AttachmentDownloadOptions) Run(cmd *cobra.Command, args []string) error {
	addr, err := FetchDownloadAddress(cmd.Context(), o.tmeet, o.MeetingID, o.FileID)
	if err != nil {
		return err
	}

	if addr.FileName == "" {
		return exception.InvalidArgsError.With(
			"server did not return file_name; please retry or verify --file-id")
	}

	// Resolve target directory; auto-create --output-dir when missing.
	dir := o.OutputDir
	if dir == "" {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			return exception.InvalidArgsError.With("get working directory: %v", cwdErr)
		}
		dir = cwd
	} else {
		info, statErr := os.Stat(dir)
		switch {
		case statErr == nil:
			if !info.IsDir() {
				return exception.InvalidArgsError.With(
					"--output-dir is not a directory: %s", dir)
			}
		case os.IsNotExist(statErr):
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return exception.InvalidArgsError.With(
					"create output directory %s: %v", dir, mkErr)
			}
		default:
			return exception.InvalidArgsError.With(
				"stat output directory %s: %v", dir, statErr)
		}
	}

	target := filepath.Join(dir, addr.FileName)

	// Fail-fast when the target already exists and --overwrite is not set.
	if !o.Overwrite {
		if _, err := os.Stat(target); err == nil {
			return exception.InvalidArgsError.With(
				"target file already exists: %s (use --overwrite to replace)", target)
		}
	}

	if err = utils.DownloadFromCOS(cmd.Context(), addr.DownloadURL, target, o.Overwrite); err != nil {
		return err
	}

	absPath, _ := filepath.Abs(target)
	data, _ := json.Marshal(map[string]interface{}{
		"file_id":   addr.FileID,
		"file_name": addr.FileName,
		"file_size": addr.FileSize,
		"file_url":  addr.DownloadURL,
		"path":      absPath,
	})
	output.FormatPrint(cmd, addr.TraceId, "attachment downloaded", string(data))
	return nil
}
