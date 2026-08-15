from fastapi.testclient import TestClient

from finance_core.main import app


def test_health_endpoint_reports_service_ready() -> None:
    client = TestClient(app)
    response = client.get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"service": "finance-core", "status": "ok"}
