package version

var (
	version = "dev"
	commit  = ""
)

func String() string {
	if commit != "" {
		return version + " (" + commit + ")"
	}
	return version
}
