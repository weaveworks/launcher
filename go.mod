module github.com/weaveworks/launcher

go 1.14

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/vcs v1.13.1 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1
	github.com/blang/semver v3.5.1+incompatible
	github.com/boltdb/bolt v1.3.1 // indirect
	github.com/certifi/gocertifi v0.0.0-20180118203423-deb3ae2ef261
	github.com/codahale/hdrhistogram v0.0.0-20161010025455-3a0bb77429bd
	github.com/davecgh/go-spew v1.1.1
	github.com/dlespiau/kube-test-harness v0.0.0-20190920101954-17e399757ec1
	github.com/docker/distribution v2.6.0-rc.1.0.20180119210003-5cb406d511b7+incompatible
	github.com/getsentry/raven-go v0.0.0-20180121060056-563b81fc02b7
	github.com/ghodss/yaml v1.0.0
	github.com/go-kit/kit v0.10.0
	github.com/go-logfmt/logfmt v0.5.0
	github.com/gogo/googleapis v1.3.1
	github.com/gogo/protobuf v1.3.1
	github.com/gogo/status v1.1.0
	github.com/golang/dep v0.5.4 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6
	github.com/golang/protobuf v1.4.2
	github.com/google/btree v1.0.0
	github.com/google/gofuzz v1.0.0
	github.com/googleapis/gnostic v0.1.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/mux v1.7.3
	github.com/gregjones/httpcache v0.0.0-20171119193500-2bcd89a1743f
	github.com/grpc-ecosystem/go-grpc-middleware v1.1.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/imdario/mergo v0.0.0-20180119215619-163f41321a19
	github.com/jessevdk/go-flags v1.3.0
	github.com/jmank88/nuts v0.4.0 // indirect
	github.com/json-iterator/go v1.1.9
	github.com/kr/logfmt v0.0.0-20140226030751-b84e30acd515
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v1.0.1
	github.com/nightlyone/lockfile v1.0.0 // indirect
	github.com/oklog/run v1.0.0
	github.com/opencontainers/go-digest v1.0.0-rc1
	github.com/opentracing-contrib/go-grpc v0.0.0-20191001143057-db30781987df
	github.com/opentracing-contrib/go-stdlib v0.0.0-20190519235532-cf7a6c988dc9
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/petar/GoLLRB v0.0.0-20130427215148-53be0d36a84c
	github.com/peterbourgon/diskv v2.0.1+incompatible
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_golang v1.4.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.9.1
	github.com/prometheus/procfs v0.0.11
	github.com/sdboyer/constext v0.0.0-20170321163424-836a14457353 // indirect
	github.com/sercand/kuberesolver v2.1.0+incompatible
	github.com/sirupsen/logrus v1.4.2
	github.com/spf13/pflag v1.0.3
	github.com/stretchr/testify v1.4.0
	github.com/uber/jaeger-client-go v2.15.0+incompatible
	github.com/uber/jaeger-lib v1.5.1-0.20181102163054-1fc5c315e03c
	github.com/weaveworks/common v0.0.0-20200904094336-dbb4d7844473
	github.com/weaveworks/promrus v1.2.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550
	golang.org/x/net v0.0.0-20200202094626-16171245cfb2
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	google.golang.org/genproto v0.0.0-20200526211855-cb27e3aa2013
	google.golang.org/grpc v1.27.0
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/inf.v0 v0.9.0
	gopkg.in/square/go-jose.v2 v2.3.1
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.0.0-20190118113203-912cbe2bfef3
	k8s.io/apiextensions-apiserver v0.0.0-20190223021643-57c81b676ab1
	k8s.io/apimachinery v0.0.0-20190223001710-c182ff3b9841
	k8s.io/apiserver v0.0.0-20190321070451-3f1a34edf9b8
	k8s.io/client-go v8.0.0+incompatible
	k8s.io/kube-openapi v0.0.0-20180108222231-a07b7bbb58e7
	k8s.io/kubernetes v1.12.0-alpha.0.0.20190501052907-9016740a6ffe
)

replace google.golang.org/grpc => google.golang.org/grpc v1.9.2
