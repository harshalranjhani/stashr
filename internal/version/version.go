package version

var (
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

func GetFullVersion() string {
	if GitCommit != "none" {
		return Version + " (" + GitCommit + ")"
	}
	return Version
}
