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

docker-seed-postgres:
	@docker run --rm -p 3030:3030 -e CLIENT="http://localhost:5173" -e JWT_SECRET="secret" -e COHERE_API_KEY="$(API_KEY)" -e POSTGRES_CONNECTION="host=host.docker.internal port=5433 user=postgres password=wc-local dbname=postgres sslmode=disable" wc-be ./wombo-combo-go-be --seed=true