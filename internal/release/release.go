package release

// Releases is the interface for querying and downloading releases.
// Wired in Task 7; declared here now to avoid an import cycle since
// machine.Context declares a Releases field.
type Releases interface {
	LatestTag(repo string) (string, error)
	Download(url string) ([]byte, error)
}
