package version

var (
	version = "dev"
	commit  = ""
	date    = ""
)

// Number returns the bare version string with no commit suffix, for
// comparison against release tags (which are compared with the "v" prefix
// trimmed).
func Number() string {
	return version
}

func Commit() string {
	return commit
}

// Date returns the ldflags-stamped build/commit date (empty in a plain build).
func Date() string {
	return date
}

func String() string {
	if commit != "" {
		return version + " (" + commit + ")"
	}
	return version
}
