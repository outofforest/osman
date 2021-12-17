module github.com/wojciech-malota-wojcik/imagebuilder

go 1.16

replace github.com/ridge/parallel => github.com/wojciech-malota-wojcik/parallel v0.1.2

// rsc.io is unreliable, take it from github
replace (
	rsc.io/binaryregexp => github.com/rsc/binaryregexp v0.2.0
	rsc.io/quote/v3 => github.com/rsc/quote/v3 v3.1.0
	rsc.io/sampler => github.com/rsc/sampler v1.3.1
)

require (
	github.com/pkg/errors v0.8.1
	github.com/ridge/must v0.6.0
	github.com/spf13/cobra v1.2.1
	github.com/wojciech-malota-wojcik/build v1.3.0
	github.com/wojciech-malota-wojcik/buildgo v0.1.1
	github.com/wojciech-malota-wojcik/ioc/v2 v2.4.0
	github.com/wojciech-malota-wojcik/isolator v0.3.0
	github.com/wojciech-malota-wojcik/libexec v0.1.1
	github.com/wojciech-malota-wojcik/logger v0.1.0
	github.com/wojciech-malota-wojcik/run v0.1.3
)
