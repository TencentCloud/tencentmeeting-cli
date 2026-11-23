package meeting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tmeet/internal"
	"tmeet/internal/cmdutil"
	"tmeet/internal/cmdutil/apicmdctx"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
	"tmeet/internal/log"
	"tmeet/internal/utils"

	restProxy "tmeet/internal/proxy/rest-proxy"
)

// Constraints for meeting description & attachments.
const (
	maxAttachmentCount       = 20
	maxTotalBytes      int64 = 100 * 1024 * 1024 // 100MB aggregate across all attachments
	maxDescChars             = 2000              // UTF-8 character count, not bytes
)

// AttachmentRef is the in-process representation of a single entry under
// `description.attachments`. It doubles as the decode target for the server
// detail response (so FileSize / FileSuffix can be preserved when round-
// tripping existing attachments through update), but it is NOT submitted
// verbatim: meeting create/update accept only file_id and file_name, so we
// project to submitAttachmentRef at the request boundary via
// toSubmitAttachmentRefs. Keeping FileSize locally also lets the update
// flow pre-check the merged aggregate size before any network call.
type AttachmentRef struct {
	FileID     string `json:"file_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size,omitempty"`
	FileSuffix string `json:"file_suffix,omitempty"`
}

// submitAttachmentRef is the on-wire shape of description.attachments[]
// accepted by meeting create/update. The server identifies an attachment
// solely by file_id; file_name is forwarded for display. Local-only fields
// carried on AttachmentRef (FileSize, FileSuffix) are intentionally
// stripped here so request bodies stay minimal.
type submitAttachmentRef struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
}

// toSubmitAttachmentRefs projects internal AttachmentRef values into the
// minimal submit DTO. A non-nil, possibly empty slice is always returned so
// callers can distinguish "clear attachments" ([]) from "no change" (nil)
// when assigning into the description payload.
func toSubmitAttachmentRefs(refs []AttachmentRef) []submitAttachmentRef {
	out := make([]submitAttachmentRef, len(refs))
	for i, r := range refs {
		out[i] = submitAttachmentRef{FileID: r.FileID, FileName: r.FileName}
	}
	return out
}

// DownloadAddress mirrors the response of the attachment download-address
// endpoint; DownloadURL has a bounded TTL on the server side and must not
// be cached across CLI invocations.
//
// TraceId is populated by FetchDownloadAddress from the HTTP response (not
// part of the server JSON payload); the `json:"-"` tag keeps it out of
// Unmarshal and any downstream re-serialization, so callers that want to
// surface it must read the field directly.
type DownloadAddress struct {
	FileID        string `json:"file_id"`
	FileName      string `json:"file_name"`
	FileSize      int64  `json:"file_size"`
	DownloadURL   string `json:"download_url"`
	URLExpireTime int64  `json:"url_expire_time"`
	TraceId       string `json:"-"`
}

// ------------------------- validation helpers -------------------------

// ValidateDescription enforces the rune-count cap for the description text.
// An empty string is considered valid (used by
// `meeting update --description ""` to clear existing text).
func ValidateDescription(text string) error {
	return utils.CharacterLimit("--description", text, maxDescChars)
}

// precheckedFile is the output of the pre-upload local scan. It bundles the
// enriched file metadata so the upload loop never re-reads the file.
type precheckedFile struct {
	path   string
	name   string
	suffix string // lower-cased, no leading dot
	size   int64
	sha1   string
	md5Hex string // used by the upload-auth request body
	md5B64 string // base64 form consumed by COS Content-MD5
	crc64  string // CRC64 ECMA decimal string, matches COS x-cos-hash-crc64ecma
}

// precheckAttachments performs ALL local-side validation BEFORE any network
// call is issued (count, aggregate size). It returns the enriched metadata
// list on success so the caller can hand it to the upload loop without a
// second disk pass.
//
// existingCount / existingBytes accommodate the update flow: when appending
// attachments to an already-populated meeting, the aggregate count and byte
// caps must still hold after the merge.
func precheckAttachments(paths []string, existingCount int, existingBytes int64) ([]precheckedFile, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	// Count precheck first so that oversized-only errors don't mask
	// trivial misconfiguration (e.g. attaching 21 files).
	if existingCount+len(paths) > maxAttachmentCount {
		return nil, exception.InvalidArgsError.With(
			"attachment count would reach %d, exceeding the limit of %d",
			existingCount+len(paths), maxAttachmentCount)
	}

	out := make([]precheckedFile, 0, len(paths))
	totalBytes := existingBytes
	for _, raw := range paths {
		p := strings.TrimSpace(raw)
		if p == "" {
			return nil, exception.InvalidArgsError.With("empty attachment path")
		}
		absPath, err := filepath.Abs(p)
		if err != nil {
			return nil, exception.InvalidArgsError.With("resolve attachment path %s: %v", p, err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, exception.InvalidArgsError.With("stat attachment %s: %v", p, err)
		}
		if info.IsDir() {
			return nil, exception.InvalidArgsError.With("attachment %s is a directory", p)
		}
		if info.Size() <= 0 {
			return nil, exception.InvalidArgsError.With("attachment %s is empty", p)
		}
		name := filepath.Base(absPath)
		suffix := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))

		totalBytes += info.Size()
		if totalBytes > maxTotalBytes {
			return nil, exception.InvalidArgsError.With(
				"attachments total size %d bytes exceeds the aggregate limit %d bytes",
				totalBytes, maxTotalBytes)
		}

		// Digest computation deferred until size/suffix precheck passes to
		// avoid hashing files that would be rejected anyway.
		digests, err := utils.CalcFileInfo(absPath)
		if err != nil {
			return nil, exception.InvalidArgsError.With("hash attachment %s: %v", p, err)
		}
		md5B64, err := utils.HexMD5ToBase64(digests.MD5)
		if err != nil {
			return nil, exception.InvalidArgsError.With("encode md5 for %s: %v", p, err)
		}
		out = append(out, precheckedFile{
			path:   absPath,
			name:   name,
			suffix: suffix,
			size:   digests.Size,
			sha1:   digests.SHA1,
			md5Hex: digests.MD5,
			md5B64: md5B64,
			crc64:  digests.CRC64,
		})
	}
	return out, nil
}

// ------------------------- server DTOs -------------------------

// cosAuthInfo is the nested COS auth signature block returned by upload-auth.
// Only put_auth is used by the attachment upload flow.
type cosAuthInfo struct {
	PutAuth string `json:"put_auth,omitempty"`
}

type uploadAuthRsp struct {
	CosURL      string      `json:"cos_url"`
	FileID      string      `json:"file_id"`
	CosAuthInfo cosAuthInfo `json:"cos_auth_info"`
}

// ------------------------- public API -------------------------

// UploadLocalAttachments executes the three-step upload flow
// (upload-auth -> PUT COS -> upload-notify) for each local path in `paths`.
// It returns the AttachmentRef slice (ready to be assigned to
// `description.attachments`) only when ALL files succeed.
//
// On the first failure it returns the error immediately so the caller never
// submits a partially-uploaded meeting. Already-uploaded file_ids are not
// cleaned up locally: the server has its own TTL for orphan files, and
// recording which uploads succeeded is useful for diagnostics.
//
// existingCount / existingBytes are 0 for the create flow; the update flow
// passes the current meeting's attachment counts to enforce the merged cap.
func UploadLocalAttachments(ctx context.Context, tmeet *internal.Tmeet,
	paths []string, existingCount int, existingBytes int64,
) ([]AttachmentRef, error) {
	files, err := precheckAttachments(paths, existingCount, existingBytes)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, nil
	}

	// Note: each sub-request (upload-auth / upload-notify) overrides the
	// api-cmd independently inside its own helper, so the parent command's
	// identifier (meeting_create / meeting_update) is not inherited by the
	// attachment sub-calls.

	refs := make([]AttachmentRef, 0, len(files))
	for _, f := range files {
		ref, err := uploadSingleAttachment(ctx, tmeet, f)
		if err != nil {
			return nil, fmt.Errorf("upload attachment %s failed: %w", f.name, err)
		}
		refs = append(refs, ref)
	}
	return refs, nil
}

// uploadSingleAttachment runs the three-step upload for exactly one file.
func uploadSingleAttachment(ctx context.Context, tmeet *internal.Tmeet,
	f precheckedFile) (AttachmentRef, error) {
	auth, err := fetchAttachmentUploadAuth(ctx, tmeet, f)
	if err != nil {
		return AttachmentRef{}, err
	}
	if auth.FileID == "" || auth.CosURL == "" || auth.CosAuthInfo.PutAuth == "" {
		return AttachmentRef{}, exception.InvalidArgsError.With(
			"upload-auth response missing required fields")
	}

	if err = utils.PutFileToCOS(ctx, f.path, auth.CosURL, auth.CosAuthInfo.PutAuth,
		f.md5B64, mimeForSuffix(f.suffix), uint64(f.size)); err != nil {
		return AttachmentRef{}, err
	}

	// upload-notify is a post-hoc server-side state sync issued only after
	// the COS object is confirmed in place; auth.FileID (returned by
	// upload-auth) is the canonical id used by description.attachments[].
	// file_id on meeting create/update. A notify failure here MUST NOT
	// fail the whole attachment upload — downgrade to a warning.
	if nerr := notifyAttachmentUpload(ctx, tmeet, auth.FileID); nerr != nil {
		log.Warnf(ctx, "notify upload success for file_id=%s swallowed: %v", auth.FileID, nerr)
	}
	return AttachmentRef{
		FileID:     auth.FileID,
		FileName:   f.name,
		FileSize:   f.size,
		FileSuffix: f.suffix,
	}, nil
}

// fetchAttachmentUploadAuth requests an upload credential for a single file.
func fetchAttachmentUploadAuth(ctx context.Context, tmeet *internal.Tmeet,
	f precheckedFile) (*uploadAuthRsp, error) {
	// Each attachment sub-call has its own api-cmd so metrics and
	// Tmeet-Cli-Name routing can distinguish upload-auth from upload-notify.
	ctx = apicmdctx.Inject(ctx, cmdutil.ApiCmdMeetingAttachmentUploadAuth)

	req := &thttp.Request{
		ApiURI: "/v1/meeting-attachments/upload-auth",
		Body: map[string]interface{}{
			"operator_id":      tmeet.UserConfig.OpenId,
			"operator_id_type": 2, // openId
			"file_info": map[string]interface{}{
				"file_name":  f.name,
				"file_type":  f.suffix,
				"file_size":  strconv.FormatInt(f.size, 10),
				"file_md5":   f.md5Hex,
				"file_sha1":  f.sha1,
				"file_crc64": f.crc64,
			},
		},
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodPost, tmeet, req)
	if err != nil {
		return nil, err
	}
	out := &uploadAuthRsp{}
	if err = json.Unmarshal([]byte(rsp.Data), out); err != nil {
		return nil, exception.InvalidArgsError.With("parse upload-auth response: %v", err)
	}
	return out, nil
}

// notifyAttachmentUpload finalizes a successfully uploaded file_id obtained
// from upload-auth. It is invoked only after the COS PUT succeeds; the
// server side has its own TTL to reap orphan file_ids left behind by
// failed uploads, so the CLI does not send a failure notification.
func notifyAttachmentUpload(ctx context.Context, tmeet *internal.Tmeet,
	fileID string) error {
	// Independent api-cmd: separates upload-notify traffic from upload-auth
	// in server-side metrics and routing.
	ctx = apicmdctx.Inject(ctx, cmdutil.ApiCmdMeetingAttachmentUploadNotify)

	req := &thttp.Request{
		ApiURI: "/v1/meeting-attachments/upload-notify",
		Body: map[string]interface{}{
			"operator_id":      tmeet.UserConfig.OpenId,
			"operator_id_type": 2,
			"file_id":          fileID,
			"file_state":       "1", // success
		},
	}
	_, err := restProxy.RequestProxy(ctx, http.MethodPost, tmeet, req)
	return err
}

// FetchDownloadAddress fetches a time-limited signed URL for the given
// attachment. Callers reached from the attachment-download subcommand will
// already have the api-cmd bound in context; the explicit Inject here is a
// safety net for other callers that invoke this function directly.
func FetchDownloadAddress(ctx context.Context, tmeet *internal.Tmeet,
	meetingID, fileID string) (*DownloadAddress, error) {
	ctx = apicmdctx.Inject(ctx, cmdutil.ApiCmdMeetingAttachmentDownload)

	q := thttp.QueryParams{}
	q.Set("operator_id", tmeet.UserConfig.OpenId)
	q.Set("operator_id_type", "2") // openId
	p := thttp.PathParams{}
	p.Set("meeting_id", meetingID)
	p.Set("file_id", fileID)

	req := &thttp.Request{
		ApiURI:      "/v1/files/meetings/{meeting_id}/attachments/{file_id}/download",
		QueryParams: q,
		PathParams:  p,
	}
	rsp, err := restProxy.RequestProxy(ctx, http.MethodGet, tmeet, req)
	if err != nil {
		return nil, err
	}
	out := &DownloadAddress{}
	if err = json.Unmarshal([]byte(rsp.Data), out); err != nil {
		return nil, exception.InvalidArgsError.With("parse download-address response: %v", err)
	}
	if out.DownloadURL == "" {
		return nil, exception.InvalidArgsError.With("download-address response missing download_url")
	}
	// Surface the backend trace id to callers (e.g. attachment-download)
	// so that the CLI's final JSON output can carry it the same way
	// RequestProxy-based commands do via rsp.TraceId.
	out.TraceId = rsp.TraceId
	return out, nil
}

// mimeForSuffix returns a best-effort Content-Type for common Office/PDF
// attachment suffixes. Unknown or missing suffixes fall back to
// octet-stream; the COS signature does not depend on this value.
func mimeForSuffix(suffix string) string {
	switch suffix {
	case "pdf":
		return "application/pdf"
	case "doc":
		return "application/msword"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	case "xls":
		return "application/vnd.ms-excel"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case "ppt":
		return "application/vnd.ms-powerpoint"
	case "pptx":
		return "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	default:
		return "application/octet-stream"
	}
}
