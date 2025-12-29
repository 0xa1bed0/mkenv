package version

var Version = "dev"

func Get() string {
	if Version == "" {
		return "dev"
	}

	return Version
}
