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


def test_request_log_line_carries_the_response_trace_id():
    import io
    import json
    import logging

    from app.main import JsonLogFormatter, logger

    stream = io.StringIO()
    handler = logging.StreamHandler(stream)
    handler.setFormatter(JsonLogFormatter())
    logger.addHandler(handler)
    try:
        response = client.post("/v1/chat/completions", json={"runtime_profile": "short"})
    finally:
        logger.removeHandler(handler)

    assert response.status_code == 200
    response_trace_id = response.json()["simulation"]["trace_id"]
    assert len(response_trace_id) == 32 and int(response_trace_id, 16) != 0

    lines = [json.loads(line) for line in stream.getvalue().splitlines()]
    access = [line for line in lines if line["path"] == "/v1/chat/completions"]
    assert len(access) == 1
    assert access[0]["message"] == "request completed"
    assert access[0]["status"] == 200
    assert access[0]["trace_id"] == response_trace_id
    assert len(access[0]["span_id"]) == 16


def test_probe_requests_are_not_access_logged():
    import io
    import logging

    from app.main import JsonLogFormatter, logger

    stream = io.StringIO()
    handler = logging.StreamHandler(stream)
    handler.setFormatter(JsonLogFormatter())
    logger.addHandler(handler)
    try:
        client.get("/health")
        client.get("/ready")
    finally:
        logger.removeHandler(handler)

    assert stream.getvalue() == ""
