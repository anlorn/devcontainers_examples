import uvicorn
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel


class Item(BaseModel):
    item_id: str
    value: str


app = FastAPI()

DATA = {}


@app.get("/{item_id}")
async def get_item(item_id: str):
    if item_id in DATA:
        return DATA[item_id]
    else:
        raise HTTPException(status_code=404, detail="Item not found")

@app.post("/")
async def set_item(item: Item):
    DATA[item.item_id] = item.value
    return {"message": "Item updated"}


def start_server():
    uvicorn.run(app, host="0.0.0.0", port=8000)


if __name__ == "__main__":
    start_server()
