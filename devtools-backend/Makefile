run:
	PORT=8080 MASTER_NODE_ENDPOINT='h2c://localhost:8090'  dotenvx run --convention=nextjs -- go run cmd/devtools/devtools.go

unittest: 
	go test -v ./...
