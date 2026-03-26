from __future__ import annotations

import asyncio
import time
import uuid
from typing import Literal

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel


DEFAULT_MODEL = "tinyllama-1.1b-chat-q4_k_m"
RUNTIME_PROFILES = {"short": 300, "medium": 1200, "long": 3500}

app = FastAPI(
    title="Service Launchpad FastAPI Service",
    version="0.3.0",
    description="A tiny chat completion simulator with fixed responses and preset runtimes.",
)


class ChatCompletionRequest(BaseModel):
    runtime_profile: Literal["short", "medium", "long"] = "medium"
    stream: bool = False


@app.get("/health")
async def health() -> dict[str, str]:
    return {"status": "ok"}


@app.get("/ready")
async def ready() -> dict[str, str]:
    return {"status": "ready"}


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
