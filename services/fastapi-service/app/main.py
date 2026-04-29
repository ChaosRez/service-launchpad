from __future__ import annotations

import asyncio
import hashlib
import os
import time
import uuid
from typing import Literal

from fastapi import FastAPI, HTTPException, Request, Response
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from pydantic import BaseModel
from prometheus_client import Counter, Histogram
from prometheus_client.registry import REGISTRY
from prometheus_client.openmetrics.exposition import CONTENT_TYPE_LATEST, generate_latest

DEFAULT_MODEL = os.getenv("FASTAPI_SERVICE_MODEL", "tinyllama-1.1b-chat-q4_k_m")
RUNTIME_PROFILES = {"short": 300, "medium": 1200, "long": 3500}
CPU_WORK_UNITS = {"short": 2_000, "medium": 25_000, "long": 120_000}
DEFAULT_RUNTIME_PROFILE = os.getenv("FASTAPI_SERVICE_DEFAULT_PROFILE", "medium")
OTEL_SERVICE_NAME = os.getenv("OTEL_SERVICE_NAME", "fastapi-service")
OTEL_EXPORTER_OTLP_TRACES_ENDPOINT = os.getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "").strip()
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
    buckets=(0.05, 0.1, 0.2, 0.3, 0.5, 1.0, 2.0, 4.0, 8.0),
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
        exemplar = _current_trace_exemplar()

        REQUEST_COUNT.labels(method=method, path=path, status=status_code).inc(exemplar=exemplar)
        REQUEST_LATENCY.labels(method=method, path=path).observe(elapsed, exemplar=exemplar)

        if response is None or response.status_code >= 400:
            ERROR_COUNT.labels(method=method, path=path, status=status_code).inc(exemplar=exemplar)


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/ready")
async def ready() -> dict[str, str]:
    return {"status": "ready"}


@app.get("/metrics")
async def metrics() -> Response:
    return Response(content=generate_latest(REGISTRY), media_type=CONTENT_TYPE_LATEST)


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

    with tracer.start_as_current_span("fastapi_service.chat_completion") as span:
        span.set_attribute("fastapi_service.runtime_profile", request.runtime_profile)

        duration_ms = RUNTIME_PROFILES[request.runtime_profile]
        await asyncio.to_thread(_do_cpu_work, CPU_WORK_UNITS[request.runtime_profile])
        await asyncio.sleep(duration_ms / 1000)

        trace_id = format(span.get_span_context().trace_id, "032x")
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
                "trace_id": trace_id,
            },
        }


def _do_cpu_work(iterations: int) -> None:
    digest = hashlib.sha256()
    for index in range(iterations):
        digest.update(f"fastapi-service-{index}".encode("utf-8"))
    digest.hexdigest()


def _current_trace_exemplar() -> dict[str, str] | None:
    span_context = trace.get_current_span().get_span_context()
    if not span_context.is_valid:
        return None
    return {"trace_id": format(span_context.trace_id, "032x")}


def _configure_telemetry():
    resource = Resource.create({"service.name": OTEL_SERVICE_NAME})
    provider = TracerProvider(resource=resource)

    if OTEL_EXPORTER_OTLP_TRACES_ENDPOINT:
        exporter = OTLPSpanExporter(endpoint=OTEL_EXPORTER_OTLP_TRACES_ENDPOINT)
        provider.add_span_processor(BatchSpanProcessor(exporter))

    trace.set_tracer_provider(provider)
    return trace.get_tracer("service-launchpad.fastapi-service")


tracer = _configure_telemetry()
FastAPIInstrumentor.instrument_app(app, excluded_urls="/health,/ready,/metrics")
