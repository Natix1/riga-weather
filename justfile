start-db:
    docker compose -f docker/development.yml up -d

stop-db:
    docker compose -f docker/development.yml down

run:
    go run .