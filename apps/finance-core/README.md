# Finance Core

Finance Core 是 Family Finance OS 唯一自研核心服务。

当前仓库只实现 `/healthz`，业务模块按 `docs/superpowers/plans/2026-08-15-v1-implementation-plan.md` 逐项 TDD 实现。

开发：

```bash
python -m venv .venv
source .venv/bin/activate
pip install -e '.[dev]'
pytest -q
uvicorn finance_core.main:app --reload --app-dir src
```
