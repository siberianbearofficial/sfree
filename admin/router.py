from fastapi import APIRouter

router = APIRouter()


@router.get("/users")
async def get_users():
    return {
        "data": [
            {
                "id": 123,
                "name": "Aleksei Orlov",
                "email": "alexeyorlov65@gmail.com",
            }
        ],
        "detail": "Users were selected."
    }
