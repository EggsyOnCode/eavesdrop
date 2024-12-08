build: 
	go build -o ./bin/eavesdrop

run: build
	./bin/eavesdrop

test: 
	go test -v ./...
