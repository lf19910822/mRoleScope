"""Collect public miHoYo program/technology social-recruitment JD responses.

This adapter starts from ``Docs/同步任务/mihoyo_weekly_sync.py``. It preserves
the established public endpoints and conservative four-worker detail fetching,
but writes raw API responses and a versioned task-directory manifest instead of
overwriting a flattened snapshot. It has no third-party Python dependencies.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import math
import os
import random
import sys
import time
import uuid
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from pathlib import Path
from typing import Any, Dict, Iterable, List, Tuple
from urllib import error, request


SCHEMA_VERSION = 1
LIST_API = "https://ats.openout.mihoyo.com/ats-portal/v1/job/list"
DETAIL_API = "https://ats.openout.mihoyo.com/ats-portal/v1/job/info"
FILTER = {
    "competencyTypes": ["1"],
    "hireType": 0,
    "channelDetailIds": [1],
}
HEADERS = {
    "Content-Type": "application/json;charset=UTF-8",
    "Accept": "application/json, text/plain, */*",
    "Origin": "https://jobs.mihoyo.com",
    "Referer": "https://jobs.mihoyo.com/",
    # Keep the User-Agent proven by the existing weekly synchronization task.
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/120.0.0.0 Safari/537.36"
    ),
}


class CollectionError(RuntimeError):
    """A completeness or transport failure that must prevent publication."""


def utc_now() -> str:
    return datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def canonical_json(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode("utf-8")


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def filter_hash() -> str:
    return sha256_bytes(canonical_json(FILTER))


def write_json(path: Path, value: Any) -> Tuple[int, str]:
    path.parent.mkdir(parents=True, exist_ok=True)
    encoded = json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2).encode("utf-8") + b"\n"
    temporary = path.with_name(path.name + ".tmp")
    temporary.write_bytes(encoded)
    os.replace(str(temporary), str(path))
    return len(encoded), sha256_bytes(encoded)


def retry_delay_seconds(attempt: int, exc: BaseException) -> float:
    """Honor an API-provided 429 delay before applying bounded backoff."""
    if isinstance(exc, error.HTTPError):
        retry_after = exc.headers.get("Retry-After") if exc.headers else None
        if retry_after:
            try:
                return min(max(float(retry_after), 0.0), 30.0)
            except ValueError:
                pass
    return min(0.5 * (2 ** (attempt - 1)), 8.0) + random.uniform(0.0, 0.4)


def post_json(url: str, payload: Dict[str, Any], retries: int, timeout_seconds: int) -> Dict[str, Any]:
    body = canonical_json(payload)
    last_error: BaseException = RuntimeError("request did not start")
    for attempt in range(1, retries + 1):
        try:
            req = request.Request(url, data=body, headers=HEADERS, method="POST")
            with request.urlopen(req, timeout=timeout_seconds) as response:
                decoded = json.loads(response.read().decode("utf-8"))
            if not isinstance(decoded, dict):
                raise CollectionError("API response is not a JSON object")
            return decoded
        except (error.URLError, error.HTTPError, TimeoutError, json.JSONDecodeError, CollectionError) as exc:
            last_error = exc
            retryable_status = not isinstance(exc, error.HTTPError) or exc.code == 429 or 500 <= exc.code <= 599
            if attempt == retries or not retryable_status:
                break
            time.sleep(retry_delay_seconds(attempt, exc))
    raise CollectionError("POST failed after {0} attempts: {1}: {2}".format(retries, url, last_error))


def list_payload(page_number: int, page_size: int) -> Dict[str, Any]:
    return dict(
        FILTER,
        jobName="",
        pageNo=page_number,
        pageSize=page_size,
    )


def detail_payload(source_job_id: str) -> Dict[str, Any]:
    # The pre-existing, working script did not send competencyTypes to the
    # detail endpoint. Do not infer undocumented fields from the list request.
    return {
        "id": str(source_job_id),
        "channelDetailIds": FILTER["channelDetailIds"],
        "hireType": FILTER["hireType"],
    }


def response_data(response: Dict[str, Any], endpoint_name: str) -> Dict[str, Any]:
    if response.get("success") is not True:
        raise CollectionError("{0} API reported failure".format(endpoint_name))
    data = response.get("data")
    if not isinstance(data, dict):
        raise CollectionError("{0} API data is not an object".format(endpoint_name))
    return data


def validate_list_pages(pages: Iterable[Dict[str, Any]]) -> List[Dict[str, Any]]:
    pages = list(pages)
    if not pages:
        raise CollectionError("list collection returned no pages")
    first_data = response_data(pages[0], "list")
    total = first_data.get("total")
    if not isinstance(total, int) or total <= 0:
        raise CollectionError("list API returned an empty or invalid total")
    items: List[Dict[str, Any]] = []
    for page in pages:
        data = response_data(page, "list")
        if data.get("total") != total or not isinstance(data.get("list"), list):
            raise CollectionError("list pagination total or item structure changed")
        items.extend(data["list"])
    ids = [str(item.get("id") or "") for item in items if isinstance(item, dict)]
    if len(items) != total or len(ids) != total or len(set(ids)) != total or "" in ids:
        raise CollectionError("list items are incomplete or contain duplicate IDs")
    return items


def validate_details(items: List[Dict[str, Any]], details: Dict[str, Dict[str, Any]]) -> List[str]:
    expected_ids = sorted(str(item["id"]) for item in items)
    if sorted(details.keys()) != expected_ids:
        raise CollectionError("detail response IDs do not match the validated list")
    for source_job_id in expected_ids:
        data = response_data(details[source_job_id], "detail")
        required = ("id", "title", "description", "jobRequire")
        if str(data.get("id")) != source_job_id or any(not data.get(field) for field in required):
            raise CollectionError("detail response is incomplete for job {0}".format(source_job_id))
    return expected_ids


def collect_live(page_size: int, workers: int, retries: int, timeout_seconds: int) -> Tuple[List[Dict[str, Any]], Dict[str, Dict[str, Any]]]:
    first = post_json(LIST_API, list_payload(1, page_size), retries, timeout_seconds)
    total = response_data(first, "list").get("total")
    if not isinstance(total, int) or total <= 0:
        raise CollectionError("list API returned an empty or invalid total")
    pages = [first]
    for page_number in range(2, int(math.ceil(float(total) / page_size)) + 1):
        pages.append(post_json(LIST_API, list_payload(page_number, page_size), retries, timeout_seconds))
    items = validate_list_pages(pages)
    details: Dict[str, Dict[str, Any]] = {}
    with ThreadPoolExecutor(max_workers=workers) as executor:
        futures = {
            executor.submit(post_json, DETAIL_API, detail_payload(str(item["id"])), retries, timeout_seconds): str(item["id"])
            for item in items
        }
        for future in as_completed(futures):
            source_job_id = futures[future]
            details[source_job_id] = future.result()
    validate_details(items, details)
    return pages, details


def probe_live(retries: int, timeout_seconds: int) -> Dict[str, Any]:
    """Verify both public endpoints in memory without creating a task directory."""
    list_data = response_data(
        post_json(LIST_API, list_payload(1, 1), retries, timeout_seconds),
        "list",
    )
    total = list_data.get("total")
    items = list_data.get("list")
    if not isinstance(total, int) or total <= 0:
        raise CollectionError("list API returned an empty or invalid total")
    if not isinstance(items, list) or len(items) != 1 or not isinstance(items[0], dict):
        raise CollectionError("list API probe response did not contain exactly one job")

    source_job_id = str(items[0].get("id") or "")
    if not source_job_id:
        raise CollectionError("list API probe response did not contain a job ID")
    detail_data = response_data(
        post_json(DETAIL_API, detail_payload(source_job_id), retries, timeout_seconds),
        "detail",
    )
    required = ("id", "title", "description", "jobRequire")
    if str(detail_data.get("id")) != source_job_id or any(not detail_data.get(field) for field in required):
        raise CollectionError("detail API probe response is incomplete")

    return {
        "detail_validated": True,
        "list_reported_total": total,
        "mode": "live-probe",
        "status": "ok",
    }


def collect_fixture(path: Path) -> Tuple[List[Dict[str, Any]], Dict[str, Dict[str, Any]]]:
    try:
        fixture = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise CollectionError("cannot read fixture: {0}".format(exc))
    pages = fixture.get("list_pages") if isinstance(fixture, dict) else None
    details = fixture.get("details") if isinstance(fixture, dict) else None
    if not isinstance(pages, list) or not all(isinstance(page, dict) for page in pages):
        raise CollectionError("fixture list_pages must be an array of raw response objects")
    if not isinstance(details, dict) or not all(isinstance(key, str) and isinstance(value, dict) for key, value in details.items()):
        raise CollectionError("fixture details must map string IDs to raw response objects")
    items = validate_list_pages(pages)
    validate_details(items, details)
    return pages, details


def write_task_directory(
    output_dir: Path,
    run_id: str,
    started_at: str,
    pages: List[Dict[str, Any]],
    details: Dict[str, Dict[str, Any]],
    retries: int,
    timeout_seconds: int,
    workers: int,
) -> Dict[str, Any]:
    files: List[Dict[str, Any]] = []
    for index, page in enumerate(pages, start=1):
        relative_path = "raw/list/page-{0:03d}.json".format(index)
        byte_count, digest = write_json(output_dir / relative_path, page)
        files.append({"path": relative_path, "sha256": digest, "bytes": byte_count})
    for source_job_id in sorted(details):
        relative_path = "raw/details/{0}.json".format(source_job_id)
        byte_count, digest = write_json(output_dir / relative_path, details[source_job_id])
        files.append({"path": relative_path, "sha256": digest, "bytes": byte_count})

    items = validate_list_pages(pages)
    manifest = {
        "schema_version": SCHEMA_VERSION,
        "status": "complete",
        "run_id": run_id,
        "started_at": started_at,
        "finished_at": utc_now(),
        "source": {
            "company": "mihoyo",
            "list_endpoint": LIST_API,
            "detail_endpoint": DETAIL_API,
            "filter": FILTER,
            "filter_sha256": filter_hash(),
        },
        "retrieval": {
            "detail_workers": workers,
            "max_attempts": retries,
            "timeout_seconds": timeout_seconds,
        },
        "counts": {
            "list_reported_total": len(items),
            "list_items": len(items),
            "unique_source_job_ids": len(items),
            "detail_succeeded": len(details),
            "detail_failed": 0,
        },
        "files": files,
    }
    write_json(output_dir / "manifest.json", manifest)
    return manifest


def run(arguments: List[str] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output-dir", help="empty task directory to create")
    parser.add_argument(
        "--probe",
        action="store_true",
        help="request one list item and its detail in memory without writing response data or files",
    )
    parser.add_argument("--run-id", default=None, help="worker-issued immutable run identity")
    parser.add_argument("--fixture", help="offline raw-response fixture; never contacts the public API")
    parser.add_argument("--page-size", type=int, default=100)
    parser.add_argument("--workers", type=int, default=4)
    parser.add_argument("--retries", type=int, default=3)
    parser.add_argument("--timeout-seconds", type=int, default=20)
    args = parser.parse_args(arguments)
    if args.page_size <= 0 or args.workers <= 0 or args.retries <= 0 or args.timeout_seconds <= 0:
        parser.error("page size, workers, retries and timeout must all be positive")

    if args.probe:
        if args.output_dir:
            parser.error("--probe cannot be combined with --output-dir")
        if args.fixture:
            parser.error("--probe always contacts the public API and cannot use --fixture")
        if args.run_id:
            parser.error("--probe does not create a run and cannot use --run-id")
        try:
            print(json.dumps(probe_live(args.retries, args.timeout_seconds), ensure_ascii=False))
            return 0
        except CollectionError as exc:
            print(
                json.dumps(
                    {"mode": "live-probe", "status": "failed", "failure": {"stage": "probe", "summary": str(exc)}},
                    ensure_ascii=False,
                ),
                file=sys.stderr,
            )
            return 1

    if not args.output_dir:
        parser.error("--output-dir is required unless --probe is used")

    output_dir = Path(args.output_dir)
    if output_dir.exists() and any(output_dir.iterdir()):
        raise CollectionError("output directory must be empty to prevent archive overwrite")
    output_dir.mkdir(parents=True, exist_ok=True)
    started_at = utc_now()
    run_id = args.run_id or str(uuid.uuid4())
    try:
        if args.fixture:
            pages, details = collect_fixture(Path(args.fixture))
        else:
            pages, details = collect_live(args.page_size, args.workers, args.retries, args.timeout_seconds)
        manifest = write_task_directory(
            output_dir, run_id, started_at, pages, details, args.retries, args.timeout_seconds, args.workers
        )
        print(json.dumps({"status": manifest["status"], "run_id": run_id, "jobs": manifest["counts"]["detail_succeeded"]}, ensure_ascii=False))
        return 0
    except CollectionError as exc:
        failure = {
            "schema_version": SCHEMA_VERSION,
            "status": "failed",
            "run_id": run_id,
            "started_at": started_at,
            "finished_at": utc_now(),
            "source": {"company": "mihoyo", "filter": FILTER, "filter_sha256": filter_hash()},
            "failure": {"stage": "collecting", "summary": str(exc)},
            "files": [],
        }
        write_json(output_dir / "manifest.json", failure)
        print(json.dumps(failure["failure"], ensure_ascii=False), file=sys.stderr)
        return 1


if __name__ == "__main__":
    try:
        sys.exit(run())
    except CollectionError as exc:
        print(str(exc), file=sys.stderr)
        sys.exit(1)
