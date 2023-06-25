package osman

import (
	"context"
	"os"
	"path/filepath"

	"github.com/digitalocean/go-libvirt"
	"github.com/pkg/errors"
	"github.com/ridge/must"
	"libvirt.org/go/libvirtxml"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
)

// Build builds image
func Build(ctx context.Context, build config.Build, s storage.Driver, builder *infra.Builder) ([]types.BuildInfo, error) {
	builds := make([]types.BuildInfo, 0, len(build.SpecFiles))
	for i, specFile := range build.SpecFiles {
		must.OK(os.Chdir(filepath.Dir(specFile)))
		buildID, err := builder.BuildFromFile(ctx, specFile, build.Names[i], build.Tags...)
		if err != nil {
			return nil, err
		}
		info, err := s.Info(ctx, buildID)
		if err != nil {
			return nil, err
		}
		builds = append(builds, info)
	}
	return builds, nil
}

// Mount mounts image
func Mount(ctx context.Context, storage config.Storage, mount config.Mount, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	properties := mount.Type.Properties()
	if !properties.Mountable {
		return types.BuildInfo{}, errors.Errorf("non-mountable image type received: %s", mount.Type)
	}

	if !mount.ImageBuildID.IsValid() {
		var err error
		mount.ImageBuildID, err = s.BuildID(ctx, mount.ImageBuildKey)
		if err != nil {
			return types.BuildInfo{}, err
		}
	}
	if !mount.ImageBuildID.Type().Properties().Cloneable {
		return types.BuildInfo{}, errors.Errorf("build %s is not cloneable", mount.ImageBuildID)
	}

	image, err := s.Info(ctx, mount.ImageBuildID)
	if err != nil {
		return types.BuildInfo{}, err
	}

	if mount.Type == types.BuildTypeBoot && len(image.Boots) == 0 {
		return types.BuildInfo{}, errors.New("image can't be mounted for booting because it was built without specifying BOOT option(s)")
	}

	if mount.MountKey.Name == "" {
		mount.MountKey.Name = image.Name
	}

	if mount.MountKey.Tag == "" {
		mount.MountKey.Tag = types.Tag(types.RandomString(5))
	}

	if !mount.MountKey.IsValid() {
		return types.BuildInfo{}, errors.Errorf("mount key %s is invalid", mount.MountKey)
	}

	info, err := cloneForMount(ctx, image, storage, mount, s)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, info.BuildID)
		}
	}()

	return info, nil
}

// Start starts VM
func Start(ctx context.Context, storage config.Storage, start config.Start, s storage.Driver) (types.BuildInfo, error) {
	var nameFromBuild bool
	if start.MountKey.Name == "" {
		nameFromBuild = true
		start.MountKey.Name = start.ImageBuildKey.Name
	}
	if start.MountKey.Tag == "" {
		start.MountKey.Tag = types.Tag(types.RandomString(5))
	}
	if start.VMFile == "" {
		start.VMFile = filepath.Join(start.XMLDir, start.MountKey.Name+".xml")
	}

	domainRaw, err := os.ReadFile(start.VMFile)
	if err != nil {
		return types.BuildInfo{}, errors.WithStack(err)
	}

	var domain libvirtxml.Domain
	if err := domain.Unmarshal(string(domainRaw)); err != nil {
		return types.BuildInfo{}, errors.WithStack(err)
	}

	if nameFromBuild {
		if domain.Name != "" {
			start.MountKey.Name = domain.Name
		}
	}

	l, err := libvirtConn(start.LibvirtAddr)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		_ = l.Disconnect()
	}()

	exists, err := vmExists(l, start.MountKey)
	if err != nil {
		return types.BuildInfo{}, err
	}
	if exists {
		return types.BuildInfo{}, errors.Errorf("vm %s has been already defined", start.MountKey)
	}

	info, err := Mount(ctx, storage, config.Mount{
		ImageBuildID:  start.ImageBuildID,
		ImageBuildKey: start.ImageBuildKey,
		MountKey:      start.MountKey,
		Type:          types.BuildTypeVM,
	}, s)
	if err != nil {
		return types.BuildInfo{}, err
	}

	if err := deployVM(ctx, l, domain, info, start); err != nil {
		return types.BuildInfo{}, err
	}
	return info, nil
}

// List lists builds
func List(ctx context.Context, filtering config.Filter, s storage.Driver) ([]types.BuildInfo, error) {
	buildTypes := map[types.BuildType]bool{}
	for _, buildType := range filtering.Types {
		buildTypes[buildType] = true
	}

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

	builds, err := s.Builds(ctx)
	if err != nil {
		return nil, err
	}
	list := make([]types.BuildInfo, 0, len(builds))
	for _, buildID := range builds {
		info, err := s.Info(ctx, buildID)
		if err != nil {
			return nil, err
		}

		if !listBuild(info, buildTypes, buildIDs, buildKeys, filtering.Untagged) {
			continue
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
func Drop(ctx context.Context, storage config.Storage, filtering config.Filter, drop config.Drop, s storage.Driver) ([]Result, error) {
	if !drop.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(ctx, filtering, s)
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
			build, err = s.Info(ctx, build.BuildID)
			if err != nil {
				return nil, err
			}
		}
	}

	if len(toDelete) == 0 {
		return nil, errors.New("no builds were selected to delete")
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

	var l *libvirt.Libvirt
	defer func() {
		if l != nil {
			_ = l.Disconnect()
		}
	}()

	results := make([]Result, 0, len(deleteSequence))
	var genGRUB bool
	for i := len(deleteSequence) - 1; i >= 0; i-- {
		buildID := deleteSequence[i]
		res := Result{BuildID: buildID}
		if buildID.Type().Properties().VM {
			if l == nil {
				var err error
				l, err = libvirtConn(drop.LibvirtAddr)
				if err != nil {
					return nil, err
				}
			}
			res.Result = undeployVM(l, buildID)
		}
		if res.Result == nil {
			res.Result = s.Drop(ctx, buildID)
		}
		if buildID.Type() == types.BuildTypeBoot && res.Result == nil {
			genGRUB = true
			res.Result = cleanKernel(buildID, "boot-")
		}

		results = append(results, res)
	}

	if genGRUB {
		if err := generateGRUB(ctx, storage, s); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// Tag removes and add tags to the build
func Tag(ctx context.Context, filtering config.Filter, tag config.Tag, s storage.Driver) ([]types.BuildInfo, error) {
	if !tag.All && len(filtering.BuildIDs) == 0 && len(filtering.BuildKeys) == 0 {
		return nil, errors.New("neither filters are provided nor All is set")
	}

	builds, err := List(ctx, filtering, s)
	if err != nil {
		return nil, err
	}

	if len(builds) == 0 {
		return nil, errors.New("no builds were selected to tag")
	}

	for _, t := range tag.Remove {
		for _, build := range builds {
			if err := s.Untag(ctx, build.BuildID, t); err != nil {
				return nil, err
			}
		}
	}
	for _, t := range tag.Add {
		for _, build := range builds {
			if err := s.Tag(ctx, build.BuildID, t); err != nil {
				return nil, err
			}
		}
	}

	filtering = config.Filter{BuildIDs: make([]types.BuildID, 0, len(builds)), Types: filtering.Types}
	for _, b := range builds {
		filtering.BuildIDs = append(filtering.BuildIDs, b.BuildID)
	}
	return List(ctx, filtering, s)
}

func listBuild(info types.BuildInfo, buildTypes map[types.BuildType]bool, buildIDs map[types.BuildID]bool, buildKeys map[types.BuildKey]bool, untagged bool) bool {
	if !buildTypes[info.BuildID.Type()] {
		return false
	}
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

func cloneForMount(ctx context.Context, image types.BuildInfo, storage config.Storage, mount config.Mount, s storage.Driver) (retInfo types.BuildInfo, retErr error) {
	buildID := types.NewBuildID(mount.Type)
	finalizeFn, buildMountpoint, err := s.Clone(ctx, image.BuildID, mount.MountKey.Name, buildID)
	if err != nil {
		return types.BuildInfo{}, err
	}
	defer func() {
		if retErr != nil {
			_ = s.Drop(ctx, buildID)
		}
	}()

	manifest := types.ImageManifest{
		BuildID: buildID,
		BasedOn: image.BuildID,
		Params:  image.Params,
	}
	if mount.Type == types.BuildTypeBoot {
		manifest.Boots = image.Boots
	}

	if err := s.StoreManifest(ctx, manifest); err != nil {
		return types.BuildInfo{}, err
	}

	if err := s.Tag(ctx, buildID, mount.MountKey.Tag); err != nil {
		return types.BuildInfo{}, err
	}

	if mount.Type == types.BuildTypeBoot {
		if err := copyKernel(buildMountpoint, storage, buildID); err != nil {
			return types.BuildInfo{}, err
		}
	}

	if err := finalizeFn(); err != nil {
		return types.BuildInfo{}, err
	}

	if mount.Type == types.BuildTypeBoot {
		if err := generateGRUB(ctx, storage, s); err != nil {
			return types.BuildInfo{}, err
		}
	}

	return s.Info(ctx, buildID)
}
