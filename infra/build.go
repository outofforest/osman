package infra

import (
	"context"
	"os"
	"path/filepath"

	"github.com/outofforest/isolator"
	"github.com/outofforest/isolator/wire"
	"github.com/outofforest/parallel"
	"github.com/pkg/errors"

	"github.com/outofforest/osman/config"
	"github.com/outofforest/osman/infra/base"
	"github.com/outofforest/osman/infra/description"
	"github.com/outofforest/osman/infra/parser"
	"github.com/outofforest/osman/infra/storage"
	"github.com/outofforest/osman/infra/types"
)

type cloneFromFn func(srcBuildKey types.BuildKey) (types.BuildInfo, error)

// NewBuilder creates new image builder
func NewBuilder(config config.Build, initializer base.Initializer, repo *Repository, storage storage.Driver, parser parser.Parser) *Builder {
	return &Builder{
		rebuild:     config.Rebuild,
		readyBuilds: map[types.BuildKey]bool{},
		initializer: initializer,
		repo:        repo,
		storage:     storage,
		parser:      parser,
	}
}

// Builder builds images
type Builder struct {
	rebuild     bool
	readyBuilds map[types.BuildKey]bool

	initializer base.Initializer
	repo        *Repository
	storage     storage.Driver
	parser      parser.Parser
}

// BuildFromFile builds image from spec file
func (b *Builder) BuildFromFile(ctx context.Context, specFile, name string, tags ...types.Tag) (types.BuildID, error) {
	return b.buildFromFile(ctx, map[types.BuildKey]bool{}, specFile, name, tags...)
}

// Build builds images
func (b *Builder) Build(ctx context.Context, img *description.Descriptor) (types.BuildID, error) {
	return b.build(ctx, map[types.BuildKey]bool{}, img)
}

func (b *Builder) buildFromFile(ctx context.Context, stack map[types.BuildKey]bool, specFile, name string, tags ...types.Tag) (types.BuildID, error) {
	commands, err := b.parser.Parse(specFile)
	if err != nil {
		return "", err
	}
	return b.build(ctx, stack, description.Describe(name, tags, commands...))
}

func (b *Builder) initialize(ctx context.Context, buildKey types.BuildKey, path string) (retErr error) {
	if buildKey.Name == "scratch" {
		return nil
	}
	// permissions on path dir has to be set to 755 to allow read access for everyone so linux boots correctly
	return b.initializer.Init(ctx, path, buildKey)
}

func (b *Builder) build(ctx context.Context, stack map[types.BuildKey]bool, img *description.Descriptor) (retBuildID types.BuildID, retErr error) {
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
			if err := os.Remove(filepath.Join(path, "root", ".specdir")); err != nil && !os.IsNotExist(err) {
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

	if len(img.Commands()) == 0 {
		if len(tags) != 1 {
			return "", errors.New("for base image exactly one tag is required")
		}

		var err error
		imgFinalize, path, err = b.storage.CreateEmpty(ctx, img.Name(), buildID)
		if err != nil {
			return "", err
		}

		if err := b.initialize(ctx, types.NewBuildKey(img.Name(), tags[0]), path); err != nil {
			return "", err
		}
	} else {
		incoming := make(chan interface{})
		outgoing := make(chan interface{})
		clonedCh := make(chan struct{})
		build := newImageBuild(incoming, outgoing,
			func(srcBuildKey types.BuildKey) (types.BuildInfo, error) {
				if !types.IsNameValid(srcBuildKey.Name) {
					return types.BuildInfo{}, errors.Errorf("name %s is invalid", srcBuildKey.Name)
				}
				if !srcBuildKey.Tag.IsValid() {
					return types.BuildInfo{}, errors.Errorf("tag %s is invalid", srcBuildKey.Tag)
				}

				// Try to clone existing image
				err := types.ErrImageDoesNotExist
				var srcBuildID types.BuildID
				if !b.rebuild || b.readyBuilds[srcBuildKey] {
					srcBuildID, err = b.storage.BuildID(ctx, srcBuildKey)
				}

				switch {
				case err == nil:
				case errors.Is(err, types.ErrImageDoesNotExist):
					// If image does not exist try to build it from file in the current directory but only if tag is a default one
					if srcBuildKey.Tag == description.DefaultTag {
						_, err = b.buildFromFile(ctx, stack, srcBuildKey.Name, srcBuildKey.Name, description.DefaultTag)
					}
				default:
					return types.BuildInfo{}, err
				}

				switch {
				case err == nil:
				case errors.Is(err, types.ErrImageDoesNotExist):
					if baseImage := b.repo.Retrieve(srcBuildKey); baseImage != nil {
						// If spec file does not exist, try building from repository
						_, err = b.build(ctx, stack, baseImage)
					} else {
						_, err = b.build(ctx, stack, description.Describe(srcBuildKey.Name, types.Tags{srcBuildKey.Tag}))
					}
				default:
					return types.BuildInfo{}, err
				}

				if err != nil {
					return types.BuildInfo{}, err
				}

				if !srcBuildID.IsValid() {
					srcBuildID, err = b.storage.BuildID(ctx, srcBuildKey)
					if err != nil {
						return types.BuildInfo{}, err
					}
				}
				if !srcBuildID.Type().Properties().Cloneable {
					return types.BuildInfo{}, errors.Errorf("build %s is not cloneable", srcBuildKey)
				}

				imgFinalize, path, err = b.storage.Clone(ctx, srcBuildID, img.Name(), buildID)
				if err != nil {
					return types.BuildInfo{}, err
				}

				buildInfo, err := b.storage.Info(ctx, srcBuildID)
				if err != nil {
					return types.BuildInfo{}, err
				}

				if err != nil {
					return types.BuildInfo{}, err
				}

				close(clonedCh)
				return buildInfo, nil
			})
		err := parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
			spawn("isolator", parallel.Fail, func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				case <-clonedCh:
				}

				return isolator.Run(ctx, isolator.Config{
					Dir: path,
					Types: []interface{}{
						wire.Result{},
						wire.Log{},
					},
					Executor: wire.Config{
						Mounts: []wire.Mount{
							{
								Host:      ".",
								Container: "/.specdir",
								Writable:  true,
							},
						},
					},
					Incoming: incoming,
					Outgoing: outgoing,
				})
			})
			spawn("commands", parallel.Exit, func(ctx context.Context) error {
				for _, cmd := range img.Commands() {
					select {
					case <-ctx.Done():
						return errors.WithStack(ctx.Err())
					default:
					}

					if err := cmd.Execute(ctx, build); err != nil {
						return err
					}
				}
				return nil
			})
			return nil
		})
		if err != nil {
			return "", err
		}

		build.manifest.BuildID = buildID
		if err := b.storage.StoreManifest(ctx, build.manifest); err != nil {
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

var _ description.ImageBuild = &imageBuild{}

func newImageBuild(incoming <-chan interface{}, outgoing chan<- interface{}, cloneFn cloneFromFn) *imageBuild {
	return &imageBuild{
		incoming: incoming,
		outgoing: outgoing,
		cloneFn:  cloneFn,
	}
}

type imageBuild struct {
	incoming <-chan interface{}
	outgoing chan<- interface{}
	cloneFn  cloneFromFn

	fromDone bool
	manifest types.ImageManifest
}

// From is a handler for FROM
func (b *imageBuild) From(cmd *description.FromCommand) error {
	if b.fromDone {
		return errors.New("directive FROM may be specified only once")
	}
	buildInfo, err := b.cloneFn(cmd.BuildKey)
	if err != nil {
		return err
	}
	b.manifest.BasedOn = buildInfo.BuildID
	b.manifest.Params = buildInfo.Params
	b.fromDone = true
	return nil
}

// Params sets kernel params for image
func (b *imageBuild) Params(cmd *description.ParamsCommand) error {
	if !b.fromDone {
		return errors.New("description has to start with FROM directive")
	}
	b.manifest.Params = append(b.manifest.Params, cmd.Params...)
	return nil
}

// Run is a handler for RUN
func (b *imageBuild) Run(ctx context.Context, cmd *description.RunCommand) error {
	if !b.fromDone {
		return errors.New("description has to start with FROM directive")
	}

	select {
	case <-ctx.Done():
		return errors.WithStack(ctx.Err())
	case b.outgoing <- wire.Execute{Command: cmd.Command}:
	}

	for {
		var content interface{}
		var ok bool

		select {
		case <-ctx.Done():
			return errors.WithStack(ctx.Err())
		case content, ok = <-b.incoming:
		}
		if !ok {
			return errors.WithStack(ctx.Err())
		}

		switch m := content.(type) {
		case wire.Log:
			stream, err := toStream(m.Stream)
			if err != nil {
				return err
			}
			if _, err := stream.WriteString(m.Text); err != nil {
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
}

// Boot sets boot option for an image
func (b *imageBuild) Boot(cmd *description.BootCommand) error {
	if !b.fromDone {
		return errors.New("description has to start with FROM directive")
	}
	b.manifest.Boots = append(b.manifest.Boots, types.Boot{Title: cmd.Title, Params: cmd.Params})
	return nil
}

func toStream(stream wire.Stream) (*os.File, error) {
	var f *os.File
	switch stream {
	case wire.StreamOut:
		f = os.Stdout
	case wire.StreamErr:
		f = os.Stderr
	default:
		return nil, errors.Errorf("unknown stream: %d", stream)
	}
	return f, nil
}
