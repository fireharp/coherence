# Django route detection produces zero routes on a real Django project

**Version:** codegraph 0.8.0

**Scope clarification (added during iteration 23 cross-validation):**
The bug is **Django-specific**, not "framework routes generally broken".
On a tinkershop test (a real Go project using `net/http.ServeMux`),
codegraph correctly detected 4 `mux.HandleFunc("GET /path", ...)`
routes. So the framework-route layer *is* wired in for the Go stdlib
mux pattern. The issue below is specifically about Django's
`urls.py` patterns.

---

## Summary

The README lists Django as a supported framework for route extraction (`path()`, `re_path()`, `url()`, `include()` in `urls.py`, with CBV `.as_view()` handling). On a 507-file Django project with textbook `urls.py` files, codegraph extracts zero `route` nodes.

## Minimal repro

Take any Django project and index it:

```bash
codegraph init --index
sqlite3 .codegraph/codegraph.db "SELECT COUNT(*) FROM nodes WHERE kind='route';"
# → 0
```

The project's `core/urls.py` contains plain Django patterns the README says are recognized:

```python
from django.contrib import admin
from django.urls import include, path
from drf_spectacular.views import (
    SpectacularAPIView, SpectacularRedocView, SpectacularSwaggerView,
)

urlpatterns = [
    path("admin/", admin.site.urls),
    path("", include("awe.urls")),
    path("schema/", SpectacularAPIView.as_view(), name="schema"),
    path("schema/swagger/", SpectacularSwaggerView.as_view(url_name="schema"), name="swagger-ui"),
    path("schema/redoc/", SpectacularRedocView.as_view(url_name="schema"), name="redoc"),
    path("", include("django_prometheus.urls")),
]
```

Three more `urls.py` files in the same project add ~20 additional `path(…)` calls. None produce `route` nodes.

The index does find references *into* `urls.py` (e.g. there are 19 references-edges pointing at urls.py file nodes), confirming codegraph processed the files. The Django pattern matcher just doesn't fire.

## Possible explanations (to investigate)

1. The extractor only matches a narrower subset of Django patterns than the README claims (e.g. requires `from django.urls import path` to be the *only* import on its line?).
2. The extractor expects the `urls.py` to be reachable from a Django `WSGI_APPLICATION` setting, and we copied only a subset of the project.
3. Regression in 0.8.0 specifically — was this verified end-to-end on Django after the last extractor refactor?

## Why this matters

The marketing benchmark in the README calls out Django explicitly (34% cheaper / 64% fewer tokens / 59% faster / 81% fewer tool calls on Django). If Django route detection isn't actually firing, that benchmark might be measuring agents that found routes via fallback grep rather than via CodeGraph itself — worth a separate verification.

For coherence specifically: this issue means `route_handler_orphans`, listed as a candidate drift meter, can't be built today on the Python side via codegraph's data alone.

## Suggested fix direction

Add an integration test against a fixture `urls.py` of the shape above. Verify each of `path(…)` / `re_path(…)` / `url(…)` / `include(…)` produces a `route` node. Verify `.as_view()` resolution links the route to the class.

## Test artifacts

- `evidence/raw/` does not include the Django DB (15 MB, omitted for size). To reproduce: any open-source Django project; clone awe-django (private) or any open Django app, copy `*/urls.py` files, and re-run `codegraph init --index`.
- `evidence/ITERATION-3.md §4` — full reality-check writeup.
