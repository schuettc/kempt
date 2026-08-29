package version

var (
	version = "dev"
	commit  = ""
)

// Number returns the bare version string with no commit suffix, for
// comparison against release tags (which are compared with the "v" prefix
// trimmed).
func Number() string {
	return version
}

func String() string {
	if commit != "" {
		return version + " (" + commit + ")"
	}
	return version
}
