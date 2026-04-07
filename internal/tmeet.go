package internal

import (
	"os"
	"tmeet/internal/common"
	"tmeet/internal/config"
	"tmeet/internal/core"
	"tmeet/internal/core/serializable"
	"tmeet/internal/core/thttp"
	"tmeet/internal/exception"
)

// Tmeet is the Tencent Meeting client.
type Tmeet struct {
	RestClient thttp.Client
	CGIClient  thttp.Client
	SystemInfo *common.SystemInfo
	UserConfig *config.UserConfig
	CLIVersion string
}

// NewTmeet creates a new Tmeet instance.
func NewTmeet() (*Tmeet, error) {
	// Create the config directory.
	configDir := config.GetConfigDir()
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, exception.InitializeFailedError.With("initial config dir:%s failed, reason:%s", configDir, err.Error())
	}

	// Create the REST client.
	customClt, err := thttp.NewClient(
		core.GetOpenEndpoint(),
		thttp.WithProtocol(thttp.DefaultHttpsProtocol),
		thttp.WithSerializer(serializable.DefaultSerializer),
		thttp.WithClient(thttp.DefaultHttpClient),
	)
	if err != nil {
		return nil, err
	}

	// Create the CGI client.
	normalClt, err := thttp.NewClient(
		core.GetCGIEndpoint(),
		thttp.WithProtocol(thttp.DefaultHttpsProtocol),
		thttp.WithSerializer(serializable.DefaultSerializer),
		thttp.WithClient(thttp.DefaultHttpClient),
	)
	if err != nil {
		return nil, err
	}

	// Get system information.
	systemInfo := common.GetSystemInfo()

	// Get user information.
	usr, err := config.GetUserConfig()
	if exception.Is(err, exception.ParseUserConfigError) ||
		exception.Is(err, exception.GetUserConfigUnknownError) {
		return nil, err
	}

	return &Tmeet{
		RestClient: customClt,
		CGIClient:  normalClt,
		SystemInfo: systemInfo,
		UserConfig: usr,
	}, nil
}
