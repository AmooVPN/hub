package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Full() string {
	if Commit == "none" && Date == "unknown" {
		return Version
	}
	return Version + " (" + Commit + " " + Date + ")"
}
