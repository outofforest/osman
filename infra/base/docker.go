package base

import (
	"errors"
	"fmt"

	"github.com/outofforest/osman/infra/types"
	"github.com/wojciech-malota-wojcik/isolator"
	"github.com/wojciech-malota-wojcik/isolator/client/wire"
)

// NewDockerInitializer creates new initializer getting base images from docker registry
func NewDockerInitializer() Initializer {
	return &dockerInitializer{}
}

type dockerInitializer struct {
}

// Initialize fetches image from docker registry and integrates it inside directory
func (f *dockerInitializer) Init(dir string, buildKey types.BuildKey) (retErr error) {
	isolator, clean, err := isolator.Start(isolator.Config{Dir: dir, Executor: wire.Config{Chroot: true}})
	if err != nil {
		return err
	}
	defer func() {
		if err := clean(); retErr == nil {
			retErr = err
		}
	}()

	if err := isolator.Send(wire.InitFromDocker{
		Image: buildKey.Name,
		Tag:   string(buildKey.Tag),
	}); err != nil {
		return err
	}
	msg, err := isolator.Receive()
	if err != nil {
		return err
	}
	result, ok := msg.(wire.Result)
	if !ok {
		return fmt.Errorf("expected Result, got: %T", msg)
	}
	if result.Error != "" {
		return errors.New(result.Error)
	}
	return nil
}
