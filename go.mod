module github.com/lucas-clemente/quic-go

go 1.14

require (
	bitbucket.com/marcmolla/gorl v0.0.0
	github.com/aunum/gold v0.0.0-20201022151355-225e849d893f
	github.com/aunum/goro v0.0.0-20200331023613-6fd962e92fb9
	github.com/aunum/log v0.0.0-20200321163253-24c356e939b0
	github.com/bifurcation/mint v0.0.0-20210616192047-fd18df995463
	github.com/gammazero/deque v0.0.0-20200124200322-7e84b94275b8
	github.com/golang/mock v1.4.3
	github.com/hashicorp/golang-lru v0.5.4
	github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f
	github.com/lucas-clemente/fnv128a v0.0.0-20160504152609-393af48d3916
	github.com/lucas-clemente/quic-clients v0.1.0
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	golang.org/x/crypto v0.0.0-20220518034528-6f7dac969898
	golang.org/x/net v0.0.0-20211112202133-69e39bad7dc2
	gonum.org/v1/gonum v0.11.0
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gorgonia.org/gorgonia v0.9.9
	gorgonia.org/tensor v0.9.4
)

replace bitbucket.com/marcmolla/gorl => ./pkg/gorl

replace github.com/aunum/gold => ./pkg/gold

replace github.com/bifurcation/mint => ./pkg/mint-a6080d464fb57a9330c2124ffb62f3c233e3400e
