build:
	@go build -o bin/wc
rebuild:	
	@rm store.db
	@go build -o bin/wc

seed:
	@./bin/wc --seed


run: build
	@./bin/wc
