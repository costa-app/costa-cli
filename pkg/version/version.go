package version

var (
	Version = "v0.0.0"
	Commit  = "none"
	Date    = "unknown"
)

func Get() string {
	return Version
}

func GetFull() string {
	return Version + " (" + Commit + ", " + Date + ")"
}
