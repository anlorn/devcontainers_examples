import uuid
import time
import pytest
from fastapi.testclient import TestClient

from python_app.main import app

@pytest.fixture
def app_client():
    """
    Fixture for test client
    """
    with TestClient(app) as client:
        yield client


def test_create_record_success(app_client):
    """
    Test succesfull new record creation
    """
    # Prepare
    item_id = f"{str(uuid.uuid4())}-{time.time()}"

    # Act
    response = app_client.post(
        "/", 
        json={"item_id": item_id, "value": "test-value"}
    )

    # Assert
    assert response.status_code == 201

def test_get_record_success(app_client):
    """
    Test succesfull record retrieval
    """
    # Prepare
    item_id = f"{str(uuid.uuid4())}-{time.time()}"
    value = str(uuid.uuid4())

    # Act
    response_post = app_client.post(
        "/", 
        json={"item_id": item_id, "value": value}
    )
    response_get = app_client.get(f'/{item_id}')

    # Assert
    assert response_post.status_code == 201
    assert response_get.status_code == 200
    assert response_get.json() == {"value": value}

def test_get_record_not_found(app_client):
    """
    Test retrieval of non existifn record
    """
    # Act
    response = app_client.get(f'/{uuid.uuid4()}')
    # Assert
    assert response.status_code == 404

def test_duplicate_create_already_exists(app_client):
    """
    Test attempt to create the same record twice
    """
    # Prepare
    item_id = f"{str(uuid.uuid4())}-{time.time()}"

    # Act
    response_first = app_client.post(
        "/", 
        json={"item_id": item_id, "value": "test-value"}
    )
    # we try to create another record with the same item_id
    response_second = app_client.post(
        "/", 
        json={"item_id": item_id, "value": "test-value"}
    )

    # Assert
    assert response_first.status_code == 201
    assert response_second.status_code == 200
