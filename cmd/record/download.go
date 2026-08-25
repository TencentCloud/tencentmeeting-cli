package record

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"tmeet/internal"
	"tmeet/internal/core/thttp"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"

	"github.com/spf13/cobra"
)

type recordFileDetail struct {
	RecordFileID            string `json:"record_file_id"`
	DownloadAddress         string `json:"download_address"`
	DownloadAddressFileType string `json:"download_address_file_type"`
}

type downloadResult struct {
	RecordFileID string `json:"record_file_id"`
	Output       string `json:"output"`
	Bytes        int64  `json:"bytes"`
	ContentType  string `json:"content_type,omitempty"`
}

// DownloadOptions holds options for downloading one recording file.
type DownloadOptions struct {
	tmeet        *internal.Tmeet
	RecordFileID string
	Output       string
}

func newDownloadCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &DownloadOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "download",
		Short: "download a recording file",
		RunE:  opts.Run,
	}
	cmd.Flags().StringVar(&opts.RecordFileID, "record-file-id", "", "record file id (required)")
	cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "output file path (default: <record-file-id>.<type>)")
	_ = cmd.MarkFlagRequired("record-file-id")
	return cmd
}

func (o *DownloadOptions) Run(cmd *cobra.Command, args []string) error {
	detail, err := o.getDetail(cmd)
	if err != nil {
		return err
	}
	if detail.DownloadAddress == "" {
		return fmt.Errorf("download address unavailable: the recording/account does not allow API downloads")
	}
	outputPath := o.Output
	if outputPath == "" {
		ext := strings.TrimPrefix(detail.DownloadAddressFileType, ".")
		if ext == "" {
			ext = "mp4"
		}
		outputPath = detail.RecordFileID + "." + ext
	}
	result, err := downloadFile(cmd, detail.DownloadAddress, outputPath, detail.RecordFileID)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(result)
	output.FormatPrint(cmd, "", "success", string(b))
	return nil
}

func (o *DownloadOptions) getDetail(cmd *cobra.Command) (*recordFileDetail, error) {
	query := thttp.QueryParams{}
	query.Set("userid", o.tmeet.UserConfig.OpenId)
	req := &thttp.Request{
		ApiURI:      "/v1/addresses/{record_file_id}",
		PathParams:  thttp.PathParams{"record_file_id": o.RecordFileID},
		QueryParams: query,
	}
	rsp, err := restProxy.RequestProxy(cmd.Context(), http.MethodGet, o.tmeet, req)
	if err != nil {
		return nil, err
	}
	detail := &recordFileDetail{}
	if err := json.Unmarshal([]byte(rsp.Data), detail); err != nil {
		return nil, fmt.Errorf("decode recording detail: %w", err)
	}
	return detail, nil
}

func downloadFile(cmd *cobra.Command, rawURL, outputPath, recordFileID string) (*downloadResult, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("invalid download address")
	}
	req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download recording: %w", err)
	}
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download recording: http status %d", rsp.StatusCode)
	}
	contentType := rsp.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "text/html") {
		return nil, fmt.Errorf("download address returned HTML instead of media")
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(outputPath), ".tmeet-download-*.part")
	if err != nil {
		return nil, err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	n, copyErr := io.Copy(tmp, rsp.Body)
	closeErr := tmp.Close()
	if copyErr != nil {
		return nil, copyErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := os.Rename(tmpName, outputPath); err != nil {
		return nil, err
	}
	return &downloadResult{RecordFileID: recordFileID, Output: outputPath, Bytes: n, ContentType: contentType}, nil
}
