DB_HOST=176.109.106.249 DB_PORT=5432 DB_NAME=postgres DB_USER=postgres DB_PASS=postgres alembic -c ../migrations/alembic.ini downgrade "$1"
