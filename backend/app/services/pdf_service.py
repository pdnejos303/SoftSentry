"""HTML (Jinja) → PDF (WeasyPrint) rendering for reports (spec 8.1 / 8.2).

`render_html` is pure Jinja and unit-testable on any platform. `render_pdf`
imports WeasyPrint lazily because its native deps (Pango / cairo) aren't present
on every dev box (notably Windows); the worker turns an ImportError into a
failed-report record rather than crashing the process. PDF generation is
exercised in the Linux container.
"""

from __future__ import annotations

from datetime import date, datetime
from pathlib import Path
from typing import Any

from jinja2 import Environment, FileSystemLoader, select_autoescape

TEMPLATES_DIR = Path(__file__).resolve().parent.parent / "reports" / "templates"

# type → template filename.
TEMPLATES = {
    "org_summary": "org_summary.html",
    "machine_detail": "machine_detail.html",
}


def _fmt_dt(value: Any) -> str:
    if isinstance(value, datetime):
        return value.strftime("%Y-%m-%d %H:%M UTC")
    if isinstance(value, date):
        return value.strftime("%Y-%m-%d")
    return "—" if value in (None, "") else str(value)


def _dash(value: Any) -> Any:
    return value if value not in (None, "") else "—"


_env = Environment(
    loader=FileSystemLoader(str(TEMPLATES_DIR)),
    autoescape=select_autoescape(["html", "xml"]),
)
_env.filters["dt"] = _fmt_dt
_env.filters["dash"] = _dash


def render_html(template: str, context: dict[str, Any]) -> str:
    return _env.get_template(template).render(**context)


def render_pdf(template: str, context: dict[str, Any]) -> bytes:
    from weasyprint import HTML  # lazy: native libs may be absent in dev

    html = render_html(template, context)
    result: bytes = HTML(string=html, base_url=str(TEMPLATES_DIR)).write_pdf()
    return result
