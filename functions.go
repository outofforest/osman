package osman

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
	"github.com/ridge/must"
)

// Build builds image
func Build(ctx context.Context, build config.Build, builder *infra.Builder) error {
	for i, specFile := range build.SpecFiles {
		must.OK(os.Chdir(filepath.Dir(specFile)))
		if err := builder.BuildFromFile(ctx, specFile, build.Names[i], build.Tags...); err != nil {
			return err
		}
	}
	return nil
}

// Mount mounts image
func Mount(mount config.Mount, s storage.Driver) error {
	if !types.IsNameValid(mount.Name) {
		return fmt.Errorf("name %s is invalid", mount.Name)
	}

	list, err := List(config.Filter{BuildKeys: []types.BuildKey{types.NewBuildKey(mount.Name, "")}}, s)
	if err != nil {
		return err
	}
	if len(list) > 0 {
		return fmt.Errorf("build %s already exists", mount.Name)
	}

	if !mount.BuildID.IsValid() {
		var err error
		mount.BuildID, err = s.BuildID(mount.BuildKey)
		if err != nil {
			return err
		}
	}
	if !mount.BuildID.Type().Properties().Cloneable {
		return fmt.Errorf("build %s is not cloneable", mount.BuildID)
	}

	srcInfo, err := s.Info(mount.BuildID)
	if err != nil {
		return err
	}

	buildID := types.NewBuildID(types.BuildTypeMount)
	if _, _, err := s.Clone(mount.BuildID, mount.Name, buildID); err != nil {
		return err
	}
	return s.StoreManifest(types.ImageManifest{
		BuildID: buildID,
		BasedOn: srcInfo.BuildID,
		Params:  srcInfo.Params,
	})
}

// List lists builds
func List(filtering config.Filter, s storage.Driver) ([]types.BuildInfo, error) {
	var buildIDs map[types.BuildID]bool
	if len(filtering.BuildIDs) > 0 {
		buildIDs = map[types.BuildID]bool{}
		for _, buildID := range filtering.BuildIDs {
			buildIDs[buildID] = true
		}
	}
	var buildKeys map[types.BuildKey]bool
	if len(filtering.BuildKeys) > 0 {
		buildKeys = map[types.BuildKey]bool{}
		for _, buildKey := range filtering.BuildKeys {
			buildKeys[buildKey] = true
		}
	}

	builds, err := s.Builds()
	if err != nil {
		return nil, err
	}
	list := make([]types.BuildInfo, 0, len(builds))
	for _, buildID := range builds {
		info, err := s.Info(buildID)
		if err != nil {
			return nil, err
		}

		if !listBuild(info, buildIDs, buildKeys, filtering.Untagged) {
			continue
		}
		if info.Mounted != "" {
			info.Mounted = filepath.Join(info.Mounted, "root")
		}
		list = append(list, info)
	}
	return list, nil
}

// Result contains error realted to build ID
type Result struct {
	BuildID types.BuildID
	Result  error
}

// Drop drops builds
func Drop(filtering config.Filter, drop config.Drop, s storage.Driver) ([]Result, error) {
	if !drop.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(filtering, s)
	if err != nil {
		return nil, err
	}

	toDelete := map[types.BuildID]bool{}
	tree := map[types.BuildID]types.BuildID{}
	for _, build := range builds {
		toDelete[build.BuildID] = true
		for {
			if _, exists := tree[build.BuildID]; exists {
				break
			}
			tree[build.BuildID] = build.BasedOn
			if build.BasedOn == "" {
				break
			}
			var err error
			build, err = s.Info(build.BuildID)
			if err != nil {
				return nil, err
			}
		}
	}

	enqueued := map[types.BuildID]bool{}
	deleteSequence := make([]types.BuildID, 0, len(builds))
	var sort func(buildID types.BuildID)
	sort = func(buildID types.BuildID) {
		if enqueued[buildID] {
			return
		}
		if baseBuildID := tree[buildID]; baseBuildID != "" {
			sort(baseBuildID)
		}
		if toDelete[buildID] {
			enqueued[buildID] = true
			deleteSequence = append(deleteSequence, buildID)
		}
	}
	for _, build := range builds {
		sort(build.BuildID)
	}

	results := make([]Result, 0, len(deleteSequence))
	for i := len(deleteSequence) - 1; i >= 0; i-- {
		buildID := deleteSequence[i]
		results = append(results, Result{BuildID: buildID, Result: s.Drop(buildID)})
	}
	return results, nil
}

func listBuild(info types.BuildInfo, buildIDs map[types.BuildID]bool, buildKeys map[types.BuildKey]bool, untagged bool) bool {
	if untagged && len(info.Tags) > 0 {
		return false
	}
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
