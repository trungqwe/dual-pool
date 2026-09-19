package buildinfo

import (
	"fmt"
	"regexp"
	"time"
)

var releaseVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:[-+][0-9A-Za-z.-]+)?$`)
var commitHash = regexp.MustCompile(`^[a-f0-9]{7,40}$`)

var (
	Product   = "poolbridge"
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
	Dirty     = "unknown"
)

type Info struct {
	Product   string
	Version   string
	Commit    string
	BuildTime string
	Dirty     string
}

func Current() Info {
	return Info{Product: Product, Version: Version, Commit: Commit, BuildTime: BuildTime, Dirty: Dirty}
}

func Format(info Info) string {
	return fmt.Sprintf("poolbridge\nversion: %s\ncommit: %s\nbuild time: %s\ndirty: %s\n",
		safeVersion(info.Version), safeCommit(info.Commit), safeBuildTime(info.BuildTime), safeDirty(info.Dirty))
}

func safeVersion(value string) string {
	if value == "dev" || releaseVersion.MatchString(value) && len(value) <= 128 {
		return value
	}
	return "unknown"
}

func safeCommit(value string) string {
	if value == "unknown" || commitHash.MatchString(value) {
		return value
	}
	return "unknown"
}

func safeBuildTime(value string) string {
	if value == "unknown" {
		return value
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil && len(value) <= 64 {
		return value
	}
	return "unknown"
}

func safeDirty(value string) string {
	if value == "true" || value == "false" {
		return value
	}
	return "unknown"
}
