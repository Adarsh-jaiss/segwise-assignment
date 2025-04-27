.phony: all build run push create-migration run-migrate

build:
	@go build -o bin/app 
run:build
	@./bin/app

push:
	@git init
	@git add .
	@git commit -s -m "$(msg)"
	@echo "pushing all files to git repository..."
	@git push origin main

create-migration:
	@echo "creating migration..."
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations -seq $$name
	@echo "migration created with name $$name"
	@echo "running migrations..."


run-migrate:
	@echo "running migrations..."
	@migrate -database "postgresql://postgres:postgres@localhost:5432/segwise?sslmode=disable" -path migrations up
