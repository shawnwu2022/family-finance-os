from fastapi import FastAPI

app = FastAPI(title="Family Finance Core", version="0.1.0")


@app.get("/healthz")
def healthz() -> dict[str, str]:
    return {"service": "finance-core", "status": "ok"}
