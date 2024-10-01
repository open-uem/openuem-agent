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
	github.com/go-co-op/gocron/v2 v2.11.0
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.37.0
)

require (
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/exp v0.0.0-20240613232115-7f521ea00fb8 // indirect
)

replace github.com/doncicuto/openuem_nats => ./internal/nats

replace github.com/doncicuto/openuem_utils => ./internal/utils
