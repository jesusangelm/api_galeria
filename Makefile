# Include variables from the .envrc file
include .envrc
###################### HELPERS ###################################
## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

############################### DEV #################################
## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api -db-dsn=${DATABASE_URL} -cors-trusted-origins=${CORS_TRUSTED_ORIGIN} -s3_bucket=${S3_BUCKET} -s3_region=${S3_REGION} -s3_endpoint=${S3_ENDPOINT} -s3_akid=${S3_ACCESS_KEY_ID} -s3_sak=${S3_SECRET_ACCESS_KEY}

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	psql ${DATABASE_URL}

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path=./migrations -database=${DATABASE_URL} up

## db/migrations/down: apply all down database migrations
.PHONY: db/migrations/down
db/migrations/down: confirm
	@echo 'Running down migrations...'
	migrate -path=./migrations -database=${DATABASE_URL} down

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

###################### AUDIT #################################
## audit: tidy dependencies and format, vet and test all code
.PHONY: audit
audit:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
	# @echo 'Formatting code...'
	# go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	# staticcheck ./...
	# @echo 'Running tests...'
	# go test -race -vet=off ./...

####################### BUILD #################################
## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api
	GOOS=linux GOARCH=amd64 go build -ldflags='-s' -o=./bin/linux_amd64/api ./cmd/api
	GOOS=linux GOARCH=arm64 go build -ldflags='-s' -o=./bin/linux_arm64/api ./cmd/api

## container/build: build the podman image of the application
.PHONY: container/build
container/build:
	@echo 'Building podman image of the API...'
	podman build -t localhost/api_galeria -f Dockerfile .

## container/pgdb_vol/create: create a podman volume for the DB
.PHONY: container/dbvol/create
container/dbvol/create:
	@echo 'Creating a volume for the DB'
	podman volume create apidb_vol

########################### RUN ##################################
## container/net/create: create a network for the app pods
.PHONY: container/net/create
container/net/create:
	@echo 'Creating the network for galeria...'
	podman network create galeria_net

## container/pod/create: create a pod for the containers
.PHONY: container/pod/create
container/pod/create:
	@echo 'Creating pods for api_galeria and db...'
	podman pod create --name apigaleria --network galeria_net -p ${API_PORT}:${API_PORT}
	podman pod create --name dbgaleria --network galeria_net -p ${PG_EXT_PORT}:5432

## docker/run/db: run a PostgreSQL podman container
.PHONY: container/run/db
container/run/db:
	@echo 'Running a container for the DB'
	podman run -d --pod dbgaleria --restart=unless-stopped --name db_api_galeria \
	-e POSTGRES_USER=${PG_USER} -e POSTGRES_PASSWORD=${PG_PASSWORD} \
	-e POSTGRES_DB=${PG_DB} -v apidb_vol:/var/lib/postgresql/data \
	docker.io/postgres:16-alpine

## docker/run/api: run a podman container using the build podman image
.PHONY: container/run/api
container/run/api:
	@echo 'Running a container of the API Application'
	podman run -d --pod apigaleria --restart=unless-stopped \
	--name api_galeria localhost/api_galeria -db-dsn=${DATABASE_URL} \
	-cors-trusted-origins=${CORS_TRUSTED_ORIGIN} -s3_bucket=${S3_BUCKET} \
	-s3_region=${S3_REGION} -s3_endpoint=${S3_ENDPOINT} \
	-s3_akid=${S3_ACCESS_KEY_ID} -s3_sak=${S3_SECRET_ACCESS_KEY} \
	-env=${ENV} -port=${API_PORT} -jwt-secret=${JWT_SECRET} -jwt-issuer=${JWT_ISSUER} \
	-jwt-audience=${JWT-AUDIENCE} -cookie-domain=${COOKIE_DOMAIN} -domain=${DOMAIN}

################ DEPLOY #################################
## galeria/build: build images/network/volume/pod neccesary for galeria app
.PHONY: galeria/build
galeria/build:
	@echo 'Building all necesary for galeria, this can take a while...'
	make container/build
	make container/dbvol/create
	make container/net/create

## galeria/deploy: create the app pod and run inside all containers related with the galeria app
.PHONY: galeria/deploy
galeria/deploy:
	@echo 'Deploying the app galeria...'
	make container/pod/create
	make container/run/db
	make container/run/api
