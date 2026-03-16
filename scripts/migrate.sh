
set -e

export $(grep -v '^#' .env | xargs)

DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@localhost:${DB_PORT}/${DB_NAME}?sslmode=disable"
MIGRATIONS_PATH="internal/adapter/postgres/migrations"

case "$1" in
    up)
        migrate -path "$MIGRATIONS_PATH" -database "$DB_URL" up
        ;;
    down)
        migrate -path "$MIGRATIONS_PATH" -database "$DB_URL" down
        ;;
    drop)
        migrate -path "$MIGRATIONS_PATH" -database "$DB_URL" drop -f
        ;;
    *)
        echo "Usage: ./scripts/migrate.sh [up|down|drop]"
        ;;
esac
