package version

// Values injected at link time by GoReleaser / -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a short human-readable build identity.
func String() string {
	if Version == "" || Version == "dev" {
		return "dev"
	}
	if Commit != "" && Commit != "none" && len(Commit) >= 7 {
		return Version + " (" + Commit[:7] + ")"
	}
	return Version
}
