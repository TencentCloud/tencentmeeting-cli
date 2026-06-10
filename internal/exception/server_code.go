package exception

const (
	ServerCodeTokenExpired   = 200190303
	ServerCodeRecordNotExist = 500277
)

// notRetryCodeMap is a map of server codes that should not be retried.
var notRetryCodeMap = map[int]bool{
	ServerCodeRecordNotExist: true,
}

// IsNotRetryCode returns true if the server code is not a retry code.
func IsNotRetryCode(code int) bool {
	return notRetryCodeMap[code]
}
