module github.com/doncicuto/openuem-agent

go 1.23.1

require (
	github.com/ceshihao/windowsupdate v0.0.4
	github.com/go-ole/go-ole v1.3.0
	github.com/scjalliance/comshim v0.0.0-20240712181150-e070933cb68e
	github.com/yusufpapurcu/wmi v1.2.4
	golang.org/x/sys v0.25.0
)

require (
	github.com/doncicuto/openuem_nats v0.0.0-00010101000000-000000000000
	github.com/doncicuto/openuem_utils v0.0.0-00010101000000-000000000000
	github.com/evangwt/go-vncproxy v1.1.0
	github.com/go-co-op/gocron/v2 v2.11.0
	github.com/google/uuid v1.6.0
	github.com/labstack/echo/v4 v4.12.0
	github.com/nats-io/nats.go v1.37.0
	github.com/ncruces/zenity v0.10.14
	golang.org/x/net v0.24.0
	gopkg.in/ini.v1 v1.67.0
)

require (
	github.com/akavel/rsrc v0.10.2 // indirect
	github.com/dchest/jsmin v0.0.0-20220218165748-59f39799265f // indirect
	github.com/evangwt/go-bufcopy v0.1.1 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josephspurrier/goversioninfo v1.4.1 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/randall77/makefat v0.0.0-20210315173500-7ddd0e42c844 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
	golang.org/x/image v0.20.0 // indirect
	golang.org/x/text v0.18.0 // indirect
)

replace github.com/doncicuto/openuem_nats => ./internal/nats

replace github.com/doncicuto/openuem_utils => ./internal/utils
