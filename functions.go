package imagebuilder

import (
	"context"
	"os"
	"path/filepath"
	"sort"

	"github.com/ridge/must"
	configBuild "github.com/wojciech-malota-wojcik/imagebuilder/commands/build/config"
	configList "github.com/wojciech-malota-wojcik/imagebuilder/commands/list/config"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/storage"
	"github.com/wojciech-malota-wojcik/imagebuilder/infra/types"
	"github.com/wojciech-malota-wojcik/logger"
	"go.uber.org/zap"
)

// Build builds image
func Build(ctx context.Context, config configBuild.Build, repo *infra.Repository, builder *infra.Builder) error {
	fedoraCmds := []infra.Command{infra.Run(`printf "nameserver 8.8.8.8\nnameserver 8.8.4.4\n" > /etc/resolv.conf`),
		infra.Run(`echo 'LANG="en_US.UTF-8"' > /etc/locale.conf`),
		infra.Run(`rm -rf /var/cache/* /tmp/*`)}

	repo.Store(infra.Describe("fedora", types.Tags{"34"}, fedoraCmds...))
	repo.Store(infra.Describe("fedora", types.Tags{"35"}, fedoraCmds...))

	for i, specFile := range config.SpecFiles {
		must.OK(os.Chdir(filepath.Dir(specFile)))

		build, err := builder.BuildFromFile(ctx, specFile, config.Names[i], config.Tags...)
		if err != nil {
			return err
		}
		logger.Get(ctx).Info("Image built", zap.Strings("params", build.Manifest().Params))
	}
	return nil
}

func listBuild(info types.BuildInfo, buildIDs map[types.BuildID]bool, buildKeys map[types.BuildKey]bool) bool {
	if buildIDs != nil && buildIDs[info.BuildID] {
		return true
	}
	if buildKeys != nil {
		if buildKeys[types.NewBuildKey(info.Name, "")] {
			return true
		}
		for _, tag := range info.Tags {
			if buildKeys[types.NewBuildKey(info.Name, tag)] || buildKeys[types.NewBuildKey("", tag)] {
				return true
			}
		}
	}
	return buildIDs == nil && buildKeys == nil
}

// List lists builds
func List(config configList.List, s storage.Driver) ([]types.BuildInfo, error) {
	var buildIDs map[types.BuildID]bool
	if len(config.BuildIDs) > 0 {
		buildIDs = map[types.BuildID]bool{}
		for _, buildID := range config.BuildIDs {
			buildIDs[buildID] = true
		}
	}
	var buildKeys map[types.BuildKey]bool
	if len(config.BuildKeys) > 0 {
		buildKeys = map[types.BuildKey]bool{}
		for _, buildKey := range config.BuildKeys {
			buildKeys[buildKey] = true
		}
	}

	builds, err := s.Builds()
	if err != nil {
		return nil, err
	}
	res := make([]types.BuildInfo, 0, len(builds))
	for _, buildID := range builds {
		info, err := s.Info(buildID)
		if err != nil {
			return nil, err
		}

		if !listBuild(info, buildIDs, buildKeys) {
			continue
		}
		sort.Sort(info.Tags)
		res = append(res, info)
	}

	sort.Slice(res, func(i int, j int) bool {
		return res[i].CreatedAt.Before(res[j].CreatedAt)
	})
	return res, nil
}
