package version

import (
	"runtime/debug"
)

const CommunityModulePath = "github.com/anzhiyu-c/anheyu-app"

const ProModulePath = "github.com/anzhiyu-c/anheyu-pro-backend"

func GetVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown (no build info)"
	}

	if buildInfo.Path == ProModulePath {
		for _, dep := range buildInfo.Deps {
			// 找到社区版核心依赖。
			if dep.Path == CommunityModulePath {
				return dep.Version
			}
		}
		return "pro (community core not found)"

	} else {
		return buildInfo.Main.Version
	}
}
