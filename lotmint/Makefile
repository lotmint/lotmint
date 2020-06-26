all: build

build:
	go build -ldflags="-X main.gitTag=unknown" -gcflags="-N -l" ./conode ./app
	#GO111MODULE=on go install -ldflags="-X main.gitTag=unknown" -gcflags="-N -l" ./conode ./byzcoin/bcadmin ./eventlog/el ./scmgr
