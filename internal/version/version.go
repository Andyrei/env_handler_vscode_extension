package version

// Version is the CLI and extension version. It can be overridden at build time:
// go build -ldflags "-X env-handler/internal/version.Version=0.1.0"
var Version = "0.1.0"
