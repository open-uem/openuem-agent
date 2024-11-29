module github.com/doncicuto/openuem-agent

go 1.23.3

require (
	github.com/ceshihao/windowsupdate v0.0.4
	github.com/go-ole/go-ole v1.3.0
	github.com/yusufpapurcu/wmi v1.2.4
	golang.org/x/sys v0.27.0
)

require (
	github.com/dgraph-io/badger/v4 v4.3.1
	github.com/doncicuto/comshim v0.0.0-20241121140116-6b86c684d9e9
	github.com/doncicuto/openuem_nats v0.0.0-00010101000000-000000000000
	github.com/doncicuto/openuem_utils v0.0.0-00010101000000-000000000000
	github.com/evangwt/go-vncproxy v1.1.0
	github.com/gliderlabs/ssh v0.3.7
	github.com/go-co-op/gocron/v2 v2.11.0
	github.com/google/uuid v1.6.0
	github.com/labstack/echo/v4 v4.12.0
	github.com/nats-io/nats.go v1.37.0
	github.com/pkg/sftp v1.13.6
	golang.org/x/crypto v0.27.0
	golang.org/x/net v0.29.0
	gopkg.in/ini.v1 v1.67.0
)

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/danieljoos/wincred v1.2.2 // indirect
	github.com/dgraph-io/ristretto v1.0.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/evangwt/go-bufcopy v0.1.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/flatbuffers v24.3.25+incompatible // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/klauspost/compress v1.17.10 // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/mod v0.22.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)

replace github.com/doncicuto/openuem_nats => ./internal/nats

replace github.com/doncicuto/openuem_utils => ./internal/utils
