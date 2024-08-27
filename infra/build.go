package infra

import (
	"context"
	"os"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/outofforest/isolator"
	"github.com/outofforest/isolator/wire"
	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/base"
	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/parser"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
)

// NewBuilder creates new image builder.
func NewBuilder(
	config config.Build,
	initializer base.Initializer,
	repo *Repository,
	storage storage.Driver,
	parser parser.Parser,
) *Builder {
	return &Builder{
		rebuild:     config.Rebuild,
		readyBuilds: map[types.BuildKey]bool{},
		initializer: initializer,
		repo:        repo,
		storage:     storage,
		parser:      parser,
	}
}

// Builder builds images.
type Builder struct {
	rebuild     bool
	readyBuilds map[types.BuildKey]bool

	initializer base.Initializer
	repo        *Repository
	storage     storage.Driver
	parser      parser.Parser
}

// BuildFromFile builds image from spec file.
func (b *Builder) BuildFromFile(
	ctx context.Context,
	cacheDir string,
	specFile,
	name string,
	tags ...types.Tag,
) (types.BuildID, error) {
	return b.buildFromFile(ctx, cacheDir, map[types.BuildKey]bool{}, specFile, name, tags...)
}

// Build builds images.
func (b *Builder) Build(ctx context.Context, cacheDir string, img *description.Descriptor) (types.BuildID, error) {
	return b.build(ctx, cacheDir, map[types.BuildKey]bool{}, img)
}

func (b *Builder) buildFromFile(
	ctx context.Context,
	cacheDir string,
	stack map[types.BuildKey]bool,
	specFile, name string,
	tags ...types.Tag,
) (types.BuildID, error) {
	commands, err := b.parser.Parse(specFile)
	if err != nil {
		return "", err
	}
	return b.build(ctx, cacheDir, stack, description.Describe(name, tags, commands...))
}

func (b *Builder) initialize(
	ctx context.Context,
	cacheDir string,
	buildKey types.BuildKey,
	path string,
) (retErr error) {
	if buildKey.Name == "scratch" {
		return nil
	}
	// Permissions on path dir has to be set to 755 to allow read access for everyone so linux boots correctly.
	return b.initializer.Init(ctx, cacheDir, path, buildKey)
}

func (b *Builder) build(
	ctx context.Context,
	cacheDir string,
	stack map[types.BuildKey]bool,
	img *description.Descriptor,
) (retBuildID types.BuildID, retErr error) {
	if !types.IsNameValid(img.Name()) {
		return "", errors.Errorf("name %s is invalid", img.Name())
	}
	tags := img.Tags()
	if len(tags) == 0 {
		tags = types.Tags{description.DefaultTag}
	}
	keys := make([]types.BuildKey, 0, len(tags))
	for _, tag := range tags {
		if !tag.IsValid() {
			return "", errors.Errorf("tag %s is invalid", tag)
		}
		key := types.NewBuildKey(img.Name(), tag)
		if stack[key] {
			return "", errors.Errorf("loop in dependencies detected on image %s", key)
		}
		stack[key] = true
		keys = append(keys, key)
	}

	buildID := types.NewBuildID(types.BuildTypeImage)

	var imgFinalize storage.FinalizeFn
	var path string
	defer func() {
		if path != "" {
			if err := os.Remove(filepath.Join(path, ".specdir")); err != nil && !os.IsNotExist(err) {
				if retErr == nil {
					retErr = err
				}
				return
			}
		}
		if imgFinalize != nil {
			if err := imgFinalize(); err != nil {
				if retErr == nil {
					retErr = err
				}
				return
			}
		}
		if retErr != nil {
			if err := b.storage.Drop(ctx, buildID); err != nil && !errors.Is(err, types.ErrImageDoesNotExist) {
				retErr = err
			}
			return
		}
	}()

	//nolint:nestif
	if commands := img.Commands(); len(commands) == 0 {
		if len(tags) != 1 {
			return "", errors.New("for base image exactly one tag is required")
		}

		var err error
		imgFinalize, path, err = b.storage.CreateEmpty(ctx, img.Name(), buildID)
		if err != nil {
			return "", err
		}

		if err := b.initialize(ctx, cacheDir, types.NewBuildKey(img.Name(), tags[0]), path); err != nil {
			return "", err
		}
	} else {
		fromCommand, ok := commands[0].(*description.FromCommand)
		if !ok {
			return "", errors.New("first command must be FROM")
		}

		var err error
		var buildInfo types.BuildInfo
		imgFinalize, path, buildInfo, err = b.clone(
			ctx,
			fromCommand.BuildKey,
			cacheDir,
			stack,
			img,
			buildID,
		)
		if err != nil {
			return "", err
		}

		err = isolator.Run(ctx, isolator.Config{
			Dir: path,
			Types: []interface{}{
				wire.Result{},
				wire.Log{},
			},
			Executor: wire.Config{
				ConfigureSystem: true,
				UseHostNetwork:  true,
				Mounts: []wire.Mount{
					{
						Host:      ".",
						Namespace: "/.specdir",
						Writable:  true,
					},
				},
			},
		}, func(ctx context.Context, incoming <-chan interface{}, outgoing chan<- interface{}) error {
			build := newImageBuild(buildInfo, incoming, outgoing)
			for _, cmd := range commands[1:] {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				default:
				}

				if err := cmd.Execute(ctx, build); err != nil {
					return err
				}
			}

			build.manifest.BuildID = buildID
			return b.storage.StoreManifest(ctx, build.manifest)
		})
		if err != nil {
			return "", err
		}
	}

	for _, key := range keys {
		if err := b.storage.Tag(ctx, buildID, key.Tag); err != nil {
			return "", err
		}
	}
	for _, key := range keys {
		b.readyBuilds[key] = true
	}
	return buildID, nil
}

func (b *Builder) clone(
	ctx context.Context,
	srcBuildKey types.BuildKey,
	cacheDir string,
	stack map[types.BuildKey]bool,
	img *description.Descriptor,
	buildID types.BuildID,
) (storage.FinalizeFn, string, types.BuildInfo, error) {
	if !types.IsNameValid(srcBuildKey.Name) {
		return nil, "", types.BuildInfo{}, errors.Errorf("name %s is invalid", srcBuildKey.Name)
	}
	if !srcBuildKey.Tag.IsValid() {
		return nil, "", types.BuildInfo{}, errors.Errorf("tag %s is invalid", srcBuildKey.Tag)
	}

	// Try to clone existing image.
	err := types.ErrImageDoesNotExist
	var srcBuildID types.BuildID
	if !b.rebuild || b.readyBuilds[srcBuildKey] {
		srcBuildID, err = b.storage.BuildID(ctx, srcBuildKey)
	}

	switch {
	case err == nil:
	case errors.Is(err, types.ErrImageDoesNotExist):
		// If image does not exist try to build it from file in the current directory but only if tag is a default one.
		if srcBuildKey.Tag == description.DefaultTag {
			_, err = b.buildFromFile(ctx, cacheDir, stack, srcBuildKey.Name, srcBuildKey.Name, description.DefaultTag)
		}
	default:
		return nil, "", types.BuildInfo{}, err
	}

	switch {
	case err == nil:
	case errors.Is(err, types.ErrImageDoesNotExist):
		if baseImage := b.repo.Retrieve(srcBuildKey); baseImage != nil {
			// If spec file does not exist, try building from repository.
			_, err = b.build(ctx, cacheDir, stack, baseImage)
		} else {
			_, err = b.build(ctx, cacheDir, stack, description.Describe(srcBuildKey.Name, types.Tags{srcBuildKey.Tag}))
		}
	default:
		return nil, "", types.BuildInfo{}, err
	}

	if err != nil {
		return nil, "", types.BuildInfo{}, err
	}

	if !srcBuildID.IsValid() {
		srcBuildID, err = b.storage.BuildID(ctx, srcBuildKey)
		if err != nil {
			return nil, "", types.BuildInfo{}, err
		}
	}
	if !srcBuildID.Type().Properties().Cloneable {
		return nil, "", types.BuildInfo{}, errors.Errorf("build %s is not cloneable", srcBuildKey)
	}

	imgFinalize, path, err := b.storage.Clone(ctx, srcBuildID, img.Name(), buildID)
	if err != nil {
		return nil, "", types.BuildInfo{}, err
	}

	buildInfo, err := b.storage.Info(ctx, srcBuildID)
	if err != nil {
		return imgFinalize, "", types.BuildInfo{}, err
	}

	if err != nil {
		return imgFinalize, "", types.BuildInfo{}, err
	}

	return imgFinalize, path, buildInfo, nil
}

var _ description.ImageBuild = &imageBuild{}

func newImageBuild(buildInfo types.BuildInfo, incoming <-chan interface{}, outgoing chan<- interface{}) *imageBuild {
	return &imageBuild{
		incoming: incoming,
		outgoing: outgoing,
		manifest: types.ImageManifest{
			BasedOn: buildInfo.BuildID,
			Params:  buildInfo.Params,
		},
	}
}

type imageBuild struct {
	incoming <-chan interface{}
	outgoing chan<- interface{}
	manifest types.ImageManifest
}

// Params sets kernel params for image.
func (b *imageBuild) Params(cmd *description.ParamsCommand) {
	b.manifest.Params = append(b.manifest.Params, cmd.Params...)
}

// Run is a handler for RUN.
func (b *imageBuild) Run(ctx context.Context, cmd *description.RunCommand) error {
	select {
	case <-ctx.Done():
		return errors.WithStack(ctx.Err())
	case b.outgoing <- wire.Execute{Command: cmd.Command}:
	}

	for content := range b.incoming {
		switch m := content.(type) {
		case wire.Log:
			if _, err := os.Stderr.Write(m.Content); err != nil {
				return err
			}
			if _, err := os.Stderr.Write([]byte{'\n'}); err != nil {
				return err
			}
		case wire.Result:
			if m.Error != "" {
				return errors.Errorf("command failed: %s", m.Error)
			}
			return nil
		default:
			return errors.New("unexpected message received")
		}
	}

	return errors.WithStack(ctx.Err())
}

// Boot sets boot option for an image.
func (b *imageBuild) Boot(cmd *description.BootCommand) {
	b.manifest.Boots = append(b.manifest.Boots, types.Boot{Title: cmd.Title, Params: cmd.Params})
}
