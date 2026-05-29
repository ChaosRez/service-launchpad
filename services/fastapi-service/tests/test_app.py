from fastapi.testclient import TestClient

from app.main import app


client = TestClient(app)


def test_health_and_ready_endpoints():
    health = client.get("/health")
    ready = client.get("/ready")

    assert health.status_code == 200
    assert health.json() == {"status": "ok"}
    assert ready.status_code == 200
    assert ready.json() == {"status": "ready"}


def test_models_endpoint_returns_default_model():
    response = client.get("/v1/models")

    assert response.status_code == 200
    body = response.json()
    assert body["data"][0]["id"]
    assert body["data"][0]["owned_by"] == "service-launchpad"


def test_chat_completion_rejects_streaming():
    response = client.post("/v1/chat/completions", json={"stream": True})

    assert response.status_code == 400
    assert response.json()["detail"] == "Streaming is not implemented in the simulator."


def test_metrics_endpoint_exposes_prometheus_metrics():
    response = client.get("/metrics")

    assert response.status_code == 200
    assert "fastapi_service_requests_total" in response.text
