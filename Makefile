build:
	@go build -o bin/wc
rebuild:	
	@rm store.db
	@go build -o bin/wc

seed:
	@./bin/wc --seed


run: build
	@./bin/wc

docker-build:
	@docker build -t wc-be .

docker-run:
	@docker run --rm -p 3030:3030 -e CLIENT="http://localhost:5173" wc-be

docker-seed:
	@docker run --rm -p 3030:3030 -e CLIENT="http://localhost:5173" -e JWT_SECRET="secret" -e COHERE_API_KEY="$(API_KEY)"  wc-be ./wombo-combo-go-be --seed=true 

docker-run-ext:
	@docker run --rm -p 3030:3030 -e CLIENT="http://localhost:5173" -e JWT_SECRET="secret" -e COHERE_API_KEY="$(API_KEY)" -e POSTGRES_CONNECTION="$(CONN_STR)" wc-be

docker-seed-ext:
	@docker run --rm -p 3030:3030 -e CLIENT="http://localhost:5173" -e JWT_SECRET="secret" -e COHERE_API_KEY="$(API_KEY)" -e POSTGRES_CONNECTION="$(CONN_STR)" wc-be ./wombo-combo-go-be --seed=true 