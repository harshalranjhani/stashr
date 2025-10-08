package version

var (
	Version   = "0.0.2"
	GitCommit = "none"
	BuildDate = "unknown"
)

func GetFullVersion() string {
	if GitCommit != "none" {
		return Version + " (" + GitCommit + ")"
	}
	return Version
}
