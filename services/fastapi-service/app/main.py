from __future__ import annotations

import asyncio
import os
import time
import uuid
from typing import Literal

from fastapi import FastAPI, HTTPException, Request, Response
from pydantic import BaseModel
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

DEFAULT_MODEL = os.getenv("FASTAPI_SERVICE_MODEL", "tinyllama-1.1b-chat-q4_k_m")
RUNTIME_PROFILES = {"short": 300, "medium": 1200, "long": 3500}
DEFAULT_RUNTIME_PROFILE = os.getenv("FASTAPI_SERVICE_DEFAULT_PROFILE", "medium")
if DEFAULT_RUNTIME_PROFILE not in RUNTIME_PROFILES:
    DEFAULT_RUNTIME_PROFILE = "medium"

app = FastAPI(
    title="Service Launchpad FastAPI Service",
    version="0.3.0",
    description="A tiny chat completion simulator with fixed responses and preset runtimes.",
)

REQUEST_COUNT = Counter(
    "fastapi_service_requests_total",
    "Total number of HTTP requests handled by the fastapi-service.",
    ["method", "path", "status"],
)
REQUEST_LATENCY = Histogram(
    "fastapi_service_request_duration_seconds",
    "Latency of HTTP requests handled by the fastapi-service.",
    ["method", "path"],
)
ERROR_COUNT = Counter(
    "fastapi_service_errors_total",
    "Total number of error responses returned by the fastapi-service.",
    ["method", "path", "status"],
)


class ChatCompletionRequest(BaseModel):
    runtime_profile: Literal["short", "medium", "long"] = DEFAULT_RUNTIME_PROFILE
    stream: bool = False


@app.middleware("http")
async def record_metrics(request: Request, call_next) -> Response:
    started_at = time.perf_counter()
    response: Response | None = None

    try:
        response = await call_next(request)
        return response
    finally:
        path = request.url.path
        method = request.method
        status_code = str(response.status_code if response else 500)
        elapsed = time.perf_counter() - started_at

        REQUEST_COUNT.labels(method=method, path=path, status=status_code).inc()
        REQUEST_LATENCY.labels(method=method, path=path).observe(elapsed)

        if response is None or response.status_code >= 400:
            ERROR_COUNT.labels(method=method, path=path, status=status_code).inc()


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/ready")
async def ready() -> dict[str, str]:
    return {"status": "ready"}


@app.get("/metrics")
async def metrics() -> Response:
    return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)


@app.get("/v1/models")
async def list_models() -> dict[str, list[dict[str, str]]]:
    return {
        "data": [
            {
                "id": DEFAULT_MODEL,
                "object": "model",
                "owned_by": "service-launchpad",
            }
        ]
    }


@app.post("/v1/chat/completions")
async def create_chat_completion(request: ChatCompletionRequest) -> dict:
    if request.stream:
        raise HTTPException(status_code=400, detail="Streaming is not implemented in the simulator.")

    duration_ms = RUNTIME_PROFILES[request.runtime_profile]
    await asyncio.sleep(duration_ms / 1000)

    return {
        "id": f"chatcmpl-{uuid.uuid4().hex[:12]}",
        "object": "chat.completion",
        "created": int(time.time()),
        "model": DEFAULT_MODEL,
        "choices": [
            {
                "index": 0,
                "message": {
                    "role": "assistant",
                    "content": "This is a simulated llama.cpp chat completion response.",
                },
                "finish_reason": "stop",
            }
        ],
        "usage": {
            "prompt_tokens": 32,
            "completion_tokens": 96,
            "total_tokens": 128,
        },
        "simulation": {
            "runtime_profile": request.runtime_profile,
            "duration_ms": duration_ms,
        },
    }
