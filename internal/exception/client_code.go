package exception

const (

	// ClientCodeInvalidArgs invalid args error
	ClientCodeInvalidArgs = 1000
	// ClientCodeInitializeFailed initialize failed error
	ClientCodeInitializeFailed = 1001
	// ClientCodeGetUserConfigUnknown get user config unknown error
	ClientCodeGetUserConfigUnknown = 1002
	// ClientCodeParseUserConfig parse user config error
	ClientCodeParseUserConfig = 1003
	// ClientCodeGetUserConfigEmpty get user config empty error
	ClientCodeGetUserConfigEmpty = 1004
	// ClientCodeUserHasBeenInitialized user has been initialized error
	ClientCodeUserHasBeenInitialized = 1005
	// ClientCodeUserIdentityExpired user identity expired error
	ClientCodeUserIdentityExpired = 1006
	// ClientCodeLogoutFailed logout failed error
	ClientCodeLogoutFailed = 1007

	// ClientCodeNetwork network error
	ClientCodeNetwork = 2000
	// ClientCodeInvalidRestApiMethod invalid rest api method error
	ClientCodeInvalidRestApiMethod = 2001
	// ClientCodeInvalidNormalApiMethod invalid normal api method error
	ClientCodeInvalidNormalApiMethod = 2002
	// ClientCodeResponseDecode response decode error
	ClientCodeResponseDecode = 2003
	// ClientCodeTokenExpired token expired error
	ClientCodeTokenExpired = 2004
	// ClientCodeRefreshTokenFailed refresh token failed error
	ClientCodeRefreshTokenFailed = 2005
	// ClientCodeRestBusiness rest business error
	ClientCodeRestBusiness = 2006
	// ClientUploadToCos upload to cos error
	ClientUploadToCos = 2007

	// ClientCodeGetAuthCode get auth_code failed error
	ClientCodeGetAuthCode = 3000
	// ClientCodeAuthorizationTimeout authorization timeout error
	ClientCodeAuthorizationTimeout = 3001
	// ClientCodeAuthorizationFailed authorization failed error
	ClientCodeAuthorizationFailed = 3002
)
