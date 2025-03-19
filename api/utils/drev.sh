DB_HOST=localhost DB_PORT=5432 DB_NAME=postgres DB_USER=postgres DB_PASS=postgres alembic -c migrations/alembic.ini downgrade "$1"

if [ $? -ne 0 ]; then
    echo -e "\n---------------------\n"
    echo "execute LOCAL_INIT=1 if you want to run on local config from local.env"
    echo -e "\n---------------------\n"
fi
