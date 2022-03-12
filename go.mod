module github.com/outofforest/osman

go 1.16

replace github.com/ridge/parallel => github.com/outofforest/parallel v0.1.2

// rsc.io is unreliable, take it from github
replace (
	rsc.io/binaryregexp => github.com/rsc/binaryregexp v0.2.0
	rsc.io/quote/v3 => github.com/rsc/quote/v3 v3.1.0
	rsc.io/sampler => github.com/rsc/sampler v1.3.1
)

require (
	github.com/beevik/etree v1.1.0
	github.com/digitalocean/go-libvirt v0.0.0-20210723161134-761cfeeb5968
	github.com/google/uuid v1.3.0
	github.com/outofforest/build v1.4.0
	github.com/outofforest/buildgo v0.2.1
	github.com/outofforest/go-zfs/v3 v3.0.0
	github.com/outofforest/ioc/v2 v2.5.0
	github.com/outofforest/isolator v0.4.2
	github.com/outofforest/libexec v0.2.1
	github.com/outofforest/logger v0.2.0
	github.com/outofforest/run v0.2.2
	github.com/pkg/errors v0.8.1
	github.com/ridge/must v0.6.0
	github.com/spf13/cobra v1.2.1
)
