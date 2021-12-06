package schema

// TOTPConfiguration represents the configuration related to TOTP options.
type TOTPConfiguration struct {
	Issuer    string `koanf:"issuer"`
	Algorithm string `koanf:"algorithm"`
	Digits    uint   `koanf:"digits"`
	Period    uint   `koanf:"period"`
	Skew      *uint  `koanf:"skew"`
}

var defaultOtpSkew = uint(1)

// DefaultTOTPConfiguration represents default configuration parameters for TOTP generation.
var DefaultTOTPConfiguration = TOTPConfiguration{
	Issuer:    "Authelia",
	Algorithm: TOTPAlgorithmSHA1,
	Digits:    6,
	Period:    30,
	Skew:      &defaultOtpSkew,
}
