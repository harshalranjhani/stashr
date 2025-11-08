package version

var (
	Version   = "0.0.3"
	GitCommit = "none"
	BuildDate = "unknown"
)

func GetFullVersion() string {
	if GitCommit != "none" {
		return Version + " (" + GitCommit + ")"
	}
	return Version
}
