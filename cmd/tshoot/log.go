package tshoot

import (
	"archive/zip"
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"tmeet/internal"
	"tmeet/internal/config"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	tmeetLog "tmeet/internal/log"
	"tmeet/internal/output"
	restProxy "tmeet/internal/proxy/rest-proxy"
	"tmeet/internal/utils"

	"github.com/spf13/cobra"
)

// LogOptions is the options for log.
type LogOptions struct {
	tmeet     *internal.Tmeet
	StartTime string // Query start time, ISO 8601, e.g. 2026-03-12T14:00+08:00
	EndTime   string // Query end time, ISO 8601, e.g. 2026-03-12T14:00+08:00
	Upload    bool   // Upload log to server, login required
}

// newLogCmd get tmeet-cli local log. to troubleshooting
func newLogCmd(tmeet *internal.Tmeet) *cobra.Command {
	opts := &LogOptions{tmeet: tmeet}
	cmd := &cobra.Command{
		Use:   "log",
		Short: "get tmeet-cli local log. to troubleshooting",
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.Run(cmd, args)
		},
	}
	cmd.Annotations = map[string]string{"skipPreCheckFlag": "upload"}

	cmd.Flags().StringVar(&opts.StartTime, "start", "", "query start time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().StringVar(&opts.EndTime, "end", "", "query end time (ISO 8601, e.g. 2026-03-12T14:00+08:00)")
	cmd.Flags().BoolVar(&opts.Upload, "upload", false, "upload log to server, login required")

	cmd.MarkFlagsRequiredTogether("start", "end")
	return cmd
}

// Run executes the log command.
func (o *LogOptions) Run(cmd *cobra.Command, args []string) error {
	timeRange, err := o.parseTimeRange(o.StartTime, o.EndTime)
	if err != nil {
		return err
	}

	logSubDir, logPrefix, logExt, logTimeFormat := tmeetLog.GetLogConfig()
	logDir := filepath.Join(config.GetConfigDir(), logSubDir)

	logFiles, err := o.collectLogFiles(logDir, logPrefix, logExt, timeRange)
	if err != nil {
		tmeetLog.Errorf(o.tmeet.TCtx, "failed to read log directory: %v", err)
		return exception.InvalidArgsError.With("failed to read log directory: %v", err)
	}

	zipPath, totalLines, err := o.packLogs(logDir, logFiles, logPrefix, logTimeFormat, timeRange)
	if err != nil {
		tmeetLog.Errorf(o.tmeet.TCtx, "failed to pack logs: %v", err)
		return exception.InvalidArgsError.With("failed to pack logs: %v", err)
	}

	if totalLines == 0 {
		output.PrintInfof(cmd, "choose time range has no log")
		return nil
	}

	if o.Upload {
		logId, err := o.uploadToServer(cmd.Context(), zipPath)
		if err != nil {
			tmeetLog.Errorf(o.tmeet.TCtx, "failed to upload log: %v", err)
			return exception.UploadToCosError.With("failed to upload log, please try again 1 minute")
		}
		output.PrintInfof(cmd, "log uploaded, log id: %s", logId)
	} else {
		output.PrintInfof(cmd, "output log saved to: %s", zipPath)
	}

	return nil
}

// timeRange represents an optional time range filter.
type timeRange struct {
	start time.Time
	end   time.Time
	valid bool // false means no filter, return all logs
}

// parseTimeRange parses --start / --end flags. Returns an unrestricted range when both are empty.
func (o *LogOptions) parseTimeRange(startStr, endStr string) (timeRange, error) {
	if startStr == "" && endStr == "" {
		return timeRange{}, nil
	}

	start, err := utils.ISO8601ToTimeStamp(startStr)
	if err != nil {
		return timeRange{}, exception.InvalidArgsError.With("--start format error: %v", err)
	}
	end, err := utils.ISO8601ToTimeStamp(endStr)
	if err != nil {
		return timeRange{}, exception.InvalidArgsError.With("--end format error: %v", err)
	}
	if end < start {
		return timeRange{}, exception.InvalidArgsError.With("--end must be after --start")
	}

	return timeRange{start: time.Unix(start, 0), end: time.Unix(end, 0), valid: true}, nil
}

// collectLogFiles enumerates the log directory and returns matching file names.
// When a time range is provided, files are pre-filtered by the date in their name.
func (o *LogOptions) collectLogFiles(logDir, logPrefix, logExt string, tr timeRange) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("log directory not found: %s", logDir)
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, logPrefix) || !strings.HasSuffix(name, logExt) {
			continue
		}
		if tr.valid && !o.logFileInRange(name, logPrefix, tr) {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}

// logFileInRange checks whether a log file may contain entries within the given time range,
// based on the date embedded in the file name.
func (o *LogOptions) logFileInRange(name, logPrefix string, tr timeRange) bool {
	rest := strings.TrimPrefix(name, logPrefix)
	if len(rest) < 10 {
		return false
	}
	// Parse the date in local timezone to align with log file naming,
	// which is based on local wall-clock date (see logging.go).
	fileDate, err := time.ParseInLocation("2006-01-02", rest[:10], time.Local)
	if err != nil {
		return false
	}
	// The file covers the whole day [fileDate, fileDate+24h); keep it if it overlaps [start, end].
	return fileDate.Add(24*time.Hour).After(tr.start) && !fileDate.After(tr.end)
}

// logFileContent holds the filtered lines of a single log file.
type logFileContent struct {
	name  string
	lines []string
}

// packLogs packs filtered log lines into a zip file and returns the zip path and total line count.
// If no lines match, no zip is created and an empty path with 0 is returned.
func (o *LogOptions) packLogs(logDir string, logFiles []string, logPrefix, logTimeFormat string, tr timeRange) (string, int, error) {
	// Collect all content first to avoid creating an empty zip.
	var contents []logFileContent
	totalLines := 0

	for _, name := range logFiles {
		lines, err := o.filterLogFile(filepath.Join(logDir, name), logTimeFormat, tr)
		if err != nil || len(lines) == 0 {
			continue
		}
		contents = append(contents, logFileContent{name: name, lines: lines})
		totalLines += len(lines)
	}

	if totalLines == 0 {
		return "", 0, nil
	}

	zipPath, err := o.createZip(logDir, contents)
	if err != nil {
		return "", 0, err
	}
	return zipPath, totalLines, nil
}

// createZip writes contents to ~/tmeet_ts_{datetime}.zip and returns the path.
// It uses a "write to temp file → rename on success" strategy to avoid leaving a corrupt zip on failure.
func (o *LogOptions) createZip(logDir string, contents []logFileContent) (string, error) {
	var zipDir string
	if o.Upload {
		// upload to cos, output to logDir
		zipDir = logDir
	} else {
		// zip to local, output to home dir
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %v", err)
		}
		zipDir = homeDir
	}

	zipPath := filepath.Join(zipDir, fmt.Sprintf("tmeet_ts_%s.zip", time.Now().Format("20060102_150405")))

	// Create the temp file in the same directory to ensure rename is atomic on the same filesystem.
	tmpFile, err := os.CreateTemp(zipDir, ".tmeet_ts_*.zip.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	// Always clean up the temp file regardless of success or failure.
	defer func() { _ = os.Remove(tmpPath) }()

	if err = o.writeZip(tmpFile, contents); err != nil {
		_ = tmpFile.Close()
		return "", err
	}
	if err = tmpFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close temp file: %v", err)
	}

	// Atomic replace: rename is atomic on the same filesystem.
	if err = os.Rename(tmpPath, zipPath); err != nil {
		return "", fmt.Errorf("failed to save zip file: %v", err)
	}
	return zipPath, nil
}

// writeZip writes log contents to w in zip format.
func (o *LogOptions) writeZip(w *os.File, contents []logFileContent) error {
	zw := zip.NewWriter(w)
	for _, fc := range contents {
		entry, err := zw.Create(fc.name)
		if err != nil {
			return fmt.Errorf("failed to create zip entry %s: %v", fc.name, err)
		}
		for _, line := range fc.lines {
			if _, err = fmt.Fprint(entry, line); err != nil {
				return fmt.Errorf("failed to write zip entry %s: %v", fc.name, err)
			}
		}
	}
	return zw.Close()
}

// filterLogFile reads a log file and filters lines by the given time range.
// If tr.valid is false, all lines are returned.
func (o *LogOptions) filterLogFile(path, logTimeFormat string, tr timeRange) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var result []string
	scanner := bufio.NewScanner(f)
	// Enlarge the per-line buffer to handle very long lines (e.g. stack traces,
	// large JSON payloads). Default is 64KB, which would otherwise cause
	// bufio.ErrTooLong and silently terminate scanning the rest of the file.
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text() + "\n"
		if !tr.valid {
			result = append(result, line)
			continue
		}
		// Log line format: 2006-01-02 15:04:05.000 [traceID] [LEVEL] caller msg
		if len(line) < len(logTimeFormat) {
			continue
		}
		t, err := time.ParseInLocation(logTimeFormat, line[:len(logTimeFormat)], time.Local)
		if err != nil {
			continue
		}
		// Inclusive on both sides: [start, end].
		if !t.Before(tr.start) && !t.After(tr.end) {
			result = append(result, line)
		}
	}
	return result, scanner.Err()
}

// uploadTokenRsp is the response body for getting upload token.
type uploadTokenRsp struct {
	LogId      string `json:"log_id"`
	UploadURL  string `json:"upload_url"`
	UploadAuth string `json:"upload_auth"`
}

// uploadToServer uploads the zip file to the server.
func (o *LogOptions) uploadToServer(ctx context.Context, zipPath string) (string, error) {
	defer func() {
		_ = os.Remove(zipPath)
	}()

	fileSize, fileHash, fileMD5, err := utils.CalcFileInfo(zipPath)
	if err != nil {
		return "", exception.InvalidArgsError.With("failed to calc file info: %v", err)
	}
	md5Bytes, err := hex.DecodeString(fileMD5)
	if err != nil {
		return "", exception.InvalidArgsError.With("failed to decode file md5: %v", err)
	}
	cosFileMD5 := base64.StdEncoding.EncodeToString(md5Bytes)

	tokenData, err := o.fetchUploadToken(ctx, zipPath, fileHash, cosFileMD5, uint64(fileSize))
	if err != nil {
		return "", err
	}

	if err = o.putFileToCOS(zipPath, tokenData.UploadURL, tokenData.UploadAuth, cosFileMD5, uint64(fileSize)); err != nil {
		return "", err
	}

	if err = o.notifyUploadComplete(ctx, tokenData.LogId); err != nil {
		return "", err
	}

	return tokenData.LogId, nil
}

// fetchUploadToken requests an upload credential from the server.
func (o *LogOptions) fetchUploadToken(ctx context.Context, zipPath, fileHash, fileMD5 string, fileSize uint64) (*uploadTokenRsp, error) {
	queryParams := thttp.QueryParams{}
	queryParams.Set("file_size", strconv.FormatUint(fileSize, 10))
	queryParams.Set("file_hash", fileHash)
	queryParams.Set("file_md5", fileMD5)
	queryParams.Set("operator_id", o.tmeet.UserConfig.OpenId)
	queryParams.Set("operator_id_type", "2") // openId
	req := &thttp.Request{
		ApiURI:      "/v1/cli/tshoot/log/upload-token",
		QueryParams: queryParams,
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodGet, o.tmeet, req)
	if err != nil {
		return nil, err
	}

	var tokenData uploadTokenRsp
	if err = json.Unmarshal([]byte(rsp.Data), &tokenData); err != nil {
		return nil, exception.InvalidArgsError.With("failed to parse upload token response: %v", err)
	}
	return &tokenData, nil
}

// notifyUploadComplete notifies the server that the file has been uploaded successfully.
func (o *LogOptions) notifyUploadComplete(ctx context.Context, logId string) error {
	req := &thttp.Request{
		ApiURI: "/v1/cli/tshoot/log/upload-complete",
		Body: map[string]interface{}{
			"log_id":           logId,
			"operator_id":      o.tmeet.UserConfig.OpenId,
			"operator_id_type": 2, // openId
		},
	}
	_, err := restProxy.RequestProxy(ctx, http.MethodPost, o.tmeet, req)
	return err
}

// putFileToCOS uploads the file at filePath to the given COS upload URL with the provided auth header.
func (o *LogOptions) putFileToCOS(filePath, uploadURL, uploadAuth, fileMD5 string, fileSize uint64) error {
	f, err := os.Open(filePath)
	if err != nil {
		return exception.InvalidArgsError.With("failed to open zip file: %v", err)
	}
	defer f.Close()

	decodeUrl, err := url.QueryUnescape(uploadURL)
	if err != nil {
		return exception.InvalidArgsError.With("failed to decode upload URL: %v", err)
	}

	req, err := http.NewRequest(http.MethodPut, decodeUrl, f)
	if err != nil {
		return exception.InvalidArgsError.With("failed to build COS request: %v", err)
	}

	// Explicitly set Request.ContentLength to prevent Go's net/http from treating
	// the length as unknown (because body is an *os.File) and falling back to
	// Transfer-Encoding: chunked, which would break COS signature verification
	// or trigger a 411 Length Required response.
	req.ContentLength = int64(fileSize)
	// Provide GetBody so the body can be re-opened on 307/308 redirects or retries.
	req.GetBody = func() (io.ReadCloser, error) {
		return os.Open(filePath)
	}

	req.Header.Set("Authorization", uploadAuth)
	req.Header.Set("Content-Type", "application/zip")
	req.Header.Set("Connection", "close")
	req.Header.Set("Content-MD5", fileMD5)

	httpClt := &http.Client{Timeout: 5 * time.Minute}
	resp, err := httpClt.Do(req)
	if err != nil {
		return exception.NetworkError.With("failed to upload zip file to COS: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return exception.UploadToCosError.With("COS upload failed, status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
