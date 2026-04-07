package thttp

// DefaultHttpsProtocol is the default HTTPS protocol.
const DefaultHttpsProtocol = "https"

// DefaultHttpProtocol is the default HTTP protocol.
const DefaultHttpProtocol = "http"

// Native and custom request headers used by the SDK.
const (
	HTTPHeaderContentType = "Content-Type"
	XTCHeaderNonce        = "X-TC-Nonce"     // Random positive integer.
	XTCHeaderAction       = "X-TC-Action"    // Interface name of the operation.
	XTCHeaderRegion       = "X-TC-Region"    // Region parameter, used to identify which region's data to operate on.
	XTCHeaderTimestamp    = "X-TC-Timestamp" // Current UNIX timestamp, records the time when the API request was initiated. For example 1529223702, in seconds.
	HeaderAccessToken     = "AccessToken"    // Token returned after successful OAuth2.0 authentication.
	HeaderOpenId          = "OpenId"         // User information after successful OAuth2.0 authentication.
)

const (
	DefaultContentType = "application/json; charset=utf-8"
)
