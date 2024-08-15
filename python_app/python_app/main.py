"""
Simple app to show usage of devcontainers in development.
For simplicity we keep whole app in one file
"""
import logging
from contextlib import asynccontextmanager
from typing import Annotated, AsyncGenerator

import uvicorn
import asyncpg
from pydantic import BaseModel
from fastapi import FastAPI, HTTPException, Request, Response, Depends, status


# For simplicity, we use basic logger even if it means
# logging is blocking
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


# Initializing FastAPI app
@asynccontextmanager
async def lifespan(app_instance: FastAPI):
    """
    Create and clean DB connections pool
    """
    app_instance.state.pool = await asyncpg.create_pool(command_timeout=10)
    logger.info("Initialized DB connection pool")
    await init_db_structure(app.state.pool)
    yield
    if getattr(app_instance.state, 'pool') and app_instance.state.pool:
        await app_instance.state.pool.close()
    logger.info("Cleaned DB connection pool")

app = FastAPI(lifespan=lifespan)


# Model
class Item(BaseModel):
    """
    Object we save and read from DB
    """
    item_id: str
    value: str


# DB section #
async def get_db_connection(
    request: Request
) -> AsyncGenerator[asyncpg.pool.PoolConnectionProxy, None]:
    """
    Acquires DB connection from the pool, intended to be used as DI
    """
    async with request.app.state.pool.acquire() as db_conn:
        logger.info("Acquired connection")
        yield db_conn
        logger.info("Released connection")


async def init_db_structure(pool: asyncpg.pool.Pool):
    """
    Simple replacement for DB migrations. Function creates DB structure
    """
    async with pool.acquire() as db_conn:
        await db_conn.execute("""
            CREATE TABLE IF NOT EXISTS data (
            id text PRIMARY KEY,
            value text);
        """)
        logger.info("Initialized DB structure")

# Custom Types
DBConnection = Annotated[asyncpg.pool.PoolConnectionProxy, Depends(get_db_connection)]


# Handlers #
@app.get("/{item_id}", status_code=status.HTTP_200_OK, responses={
    "200": {"description": "Item found"},
    "404": {"description": "Item not found"}
})
async def get_item(item_id: str, db_conn: DBConnection):
    """
    Get item value by `item_id`
    """
    res = await db_conn.fetchrow(
        'SELECT value from data WHERE id = $1', item_id
    )
    if not res:
        raise HTTPException(status_code=404, detail="Item not found")
    # we fetch using table PK, so it is safe to assume we have only one record
    return {"value": next(res.items())[1]}


@app.post("/", status_code=status.HTTP_201_CREATED, responses={
    "201": {"description": "Item was created"},
    "200": {"description": "Item with such `item_id` already exists"}
})
async def set_item(item: Item, db_conn: DBConnection):
    """
    Creates new item if it doesn't exist
    """
    res = await db_conn.execute(
        "INSERT INTO data (id, value) VALUES ($1, $2) ON CONFLICT (id) DO NOTHING",
        item.item_id, item.value
    )
    if res.endswith("0 0"):
        logger.info("Item %s already exists, do nothing", item.item_id)
        return Response(status_code=status.HTTP_200_OK)
    logger.info("Created item %s with value %s", item.item_id, item.value)
    return None


# HTTP Server #
def start_server():
    """
    Started uvicorn server
    """
    uvicorn.run('main:app', host="0.0.0.0", port=8000, reload=True)


if __name__ == "__main__":
    start_server()
