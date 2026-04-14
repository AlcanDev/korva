package version

// These variables are set at build time via -ldflags.
// Example:
//
//	go build -ldflags "-X github.com/alcandev/korva/internal/version.Version=1.0.0 \
//	  -X github.com/alcandev/korva/internal/version.Commit=abc123 \
//	  -X github.com/alcandev/korva/internal/version.Date=2026-04-13"
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a human-readable version string.
func String() string {
	return Version + " (" + Commit + ") built " + Date
}
