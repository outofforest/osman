module github.com/outofforest/osman

go 1.20

// rsc.io is unreliable, take it from github
replace (
	rsc.io/binaryregexp => github.com/rsc/binaryregexp v0.2.0
	rsc.io/quote/v3 => github.com/rsc/quote/v3 v3.1.0
	rsc.io/sampler => github.com/rsc/sampler v1.3.1
)

require (
	github.com/beevik/etree v1.2.0
	github.com/digitalocean/go-libvirt v0.0.0-20221205150000-2939327a8519
	github.com/google/uuid v1.3.0
	github.com/outofforest/go-zfs/v3 v3.1.14
	github.com/outofforest/ioc/v2 v2.5.2
	github.com/outofforest/isolator v0.5.6
	github.com/outofforest/logger v0.3.4
	github.com/outofforest/run v0.3.1
	github.com/pkg/errors v0.9.1
	github.com/ridge/must v0.6.0
	github.com/spf13/cobra v1.7.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/outofforest/libexec v0.3.8 // indirect
	github.com/outofforest/parallel v0.2.3 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
)
