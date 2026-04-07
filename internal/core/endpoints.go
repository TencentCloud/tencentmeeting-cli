package core

// Endpoints holds the endpoint configuration.
type Endpoints struct {
	Open string // Open API address
	CGI  string // CGI address
	Auth string // Auth address
}

// baseEndpoints returns the base endpoint configuration.
func baseEndpoints() *Endpoints {
	return &Endpoints{
		Open: "api.meeting.qq.com",
		CGI:  "work.medialab.qq.com",
		Auth: "meeting.tencent.com",
	}
}

// GetOpenEndpoint returns the Open API address.
func GetOpenEndpoint() string {
	return baseEndpoints().Open
}

// GetAuthEndpoint returns the Auth address.
func GetAuthEndpoint() string {
	return baseEndpoints().Auth
}

// GetCGIEndpoint returns the CGI address.
func GetCGIEndpoint() string {
	return baseEndpoints().CGI
}
