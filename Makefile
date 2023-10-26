.PHONY: all build dev prod

all: build dev

build:
	@echo "Building backend..."
	go build -o rf-server-go

dev:
	@echo "Starting dev backend..."
	./rf-server-go .env.dev &

prod:
	@echo "Starting prod backend..."
	./rf-server-go .env &

stop:
	@echo "Stopping proccesses..."
	@pkill -f "./rf-server-go"
	