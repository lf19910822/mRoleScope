"""
Concurrent crawler for public miHoYo social-recruitment program/tech jobs.

Captured public APIs:
- POST https://ats.openout.mihoyo.com/ats-portal/v1/job/list
- POST https://ats.openout.mihoyo.com/ats-portal/v1/job/info

No login, no CAPTCHA bypass, no private data.

Recommended:
    py crawl_mihoyo_jobs_concurrent.py --page-size 100 --workers 12 --retries 5

If you see HTTP 429 / 403 / many timeouts, lower --workers, e.g. --workers 5.
"""

import argparse
import csv
import html
import json
import math
import random
import re
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime
from pathlib import Path
from threading import Lock
from typing import Any, Dict, List, Optional, Tuple
from urllib import error, request


LIST_API = "https://ats.openout.mihoyo.com/ats-portal/v1/job/list"
INFO_API = "https://ats.openout.mihoyo.com/ats-portal/v1/job/info"
REFERER = "https://jobs.mihoyo.com/"

DEFAULT_PAGE_SIZE = 100
DEFAULT_WORKERS = 12
DEFAULT_RETRIES = 5
DEFAULT_TIMEOUT_SECONDS = 20
DEFAULT_DELAY_SECONDS = 0.0


HEADERS = {
    "Content-Type": "application/json;charset=UTF-8",
    "Accept": "application/json, text/plain, */*",
    "Origin": "https://jobs.mihoyo.com",
    "Referer": REFERER,
    "User-Agent": (
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
        "AppleWebKit/537.36 (KHTML, like Gecko) "
        "Chrome/120.0.0.0 Safari/537.36"
    ),
}

# Avoid interleaved output in concurrent mode.
PRINT_LOCK = Lock()


def log(message: str) -> None:
    with PRINT_LOCK:
        print(message, flush=True)


def retry_sleep(attempt: int, exc: BaseException) -> None:
    """Exponential backoff with jitter. Honors Retry-After for HTTP 429 when available."""
    retry_after: Optional[str] = None
    if isinstance(exc, error.HTTPError):
        retry_after = exc.headers.get("Retry-After")

    if retry_after:
        try:
            seconds = float(retry_after)
            time.sleep(min(seconds, 30.0))
            return
        except ValueError:
            pass

    # 0.5, 1, 2, 4, 8... capped, plus jitter.
    base = min(0.5 * (2 ** (attempt - 1)), 8.0)
    jitter = random.uniform(0.0, 0.4)
    time.sleep(base + jitter)


def should_retry_http_status(status: int) -> bool:
    # 429 = too many requests; 5xx = server / gateway transient errors.
    return status == 429 or 500 <= status <= 599


def post_json(
    url: str,
    payload: Dict[str, Any],
    retries: int = DEFAULT_RETRIES,
    timeout: int = DEFAULT_TIMEOUT_SECONDS,
) -> Dict[str, Any]:
    body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
    last_exc: Optional[BaseException] = None

    for attempt in range(1, retries + 1):
        try:
            req = request.Request(url=url, data=body, headers=HEADERS, method="POST")
            with request.urlopen(req, timeout=timeout) as resp:
                text = resp.read().decode("utf-8")
                return json.loads(text)

        except error.HTTPError as exc:
            last_exc = exc
            # Retry only on transient/rate-limit statuses.
            if attempt < retries and should_retry_http_status(exc.code):
                log(f"[retry] HTTP {exc.code} {url}; attempt {attempt}/{retries}")
                retry_sleep(attempt, exc)
                continue
            break

        except (error.URLError, TimeoutError, json.JSONDecodeError) as exc:
            last_exc = exc
            if attempt < retries:
                log(f"[retry] {type(exc).__name__} {url}; attempt {attempt}/{retries}")
                retry_sleep(attempt, exc)
                continue
            break

    raise RuntimeError(f"POST failed after {retries} attempts: {url}, payload={payload}, error={last_exc}")


def clean_text(value: Any) -> str:
    if value is None:
        return ""
    text = str(value)
    text = html.unescape(text)
    text = re.sub(r"<br\s*/?>", "\n", text, flags=re.IGNORECASE)
    text = re.sub(r"<[^>]+>", "", text)
    text = text.replace("\r\n", "\n").replace("\r", "\n")
    text = re.sub(r"[ \t]+\n", "\n", text)
    text = re.sub(r"\n{3,}", "\n\n", text)
    return text.strip()


def address_text(job: Dict[str, Any]) -> str:
    addresses = job.get("addressDetailList") or []
    return "、".join(clean_text(x.get("addressDetail")) for x in addresses if x.get("addressDetail"))


def fetch_job_list(page_no: int, page_size: int, retries: int, timeout: int) -> Dict[str, Any]:
    payload = {
        "jobName": "",
        "competencyTypes": ["1"],  # 1 = 程序&技术类；来自官网请求参数
        "pageNo": page_no,
        "pageSize": page_size,
        "channelDetailIds": [1],
        "hireType": 0,  # 0 = 社会招聘
    }
    resp = post_json(LIST_API, payload, retries=retries, timeout=timeout)
    if not resp.get("success"):
        raise RuntimeError(f"list api returned error: {resp}")
    return resp.get("data") or {}


def fetch_job_info(job_id: str, retries: int, timeout: int) -> Dict[str, Any]:
    payload = {
        "id": str(job_id),
        "channelDetailIds": [1],
        "hireType": 0,
    }
    resp = post_json(INFO_API, payload, retries=retries, timeout=timeout)
    if not resp.get("success"):
        raise RuntimeError(f"info api returned error for job_id={job_id}: {resp}")
    return resp.get("data") or {}


def normalize_job(list_item: Dict[str, Any], detail: Dict[str, Any]) -> Dict[str, Any]:
    source = {**list_item, **detail}
    job_id = clean_text(source.get("id"))

    return {
        "id": job_id,
        "title": clean_text(source.get("title")),
        "addresses": address_text(source),
        "competency_type": clean_text(source.get("competencyType")),
        "competency_type_id": clean_text(source.get("competencyTypeId")),
        "job_nature": clean_text(source.get("jobNature")),
        "job_nature_id": source.get("jobNatureId"),
        "project_name": clean_text(source.get("projectName")),
        "hire_type_name": clean_text(source.get("hireTypeName")),
        "object_name": clean_text(source.get("objectName")),
        "hurry": bool(source.get("hurry")),
        "description": clean_text(source.get("description")),
        "job_require": clean_text(source.get("jobRequire")),
        "addition": clean_text(source.get("addition")),
        "delivery_instructions": clean_text(source.get("deliveryInstructions")),
        "job_summary": clean_text(source.get("jobSummary")),
        # 详情页路由若官网后续调整，此字段仅作为辅助；权威信息以 id + API 结果为准。
        "candidate_url": f"https://jobs.mihoyo.com/#/position/{job_id}" if job_id else "",
    }



def collect():
    page_size = 100
    first = fetch_job_list(1, page_size, 3, 20)
    total = int(first.get("total", 0))
    if total <= 0:
        raise RuntimeError("Empty result; preserve previous successful snapshot")
    items = list(first.get("list") or [])
    for page in range(2, math.ceil(total / page_size) + 1):
        part = fetch_job_list(page, page_size, 3, 20)
        if int(part.get("total", -1)) != total:
            raise RuntimeError("List changed during pagination; preserve previous snapshot")
        items.extend(part.get("list") or [])
    ids = [str(x.get("id") or "") for x in items]
    if len(items) != total or len(set(ids)) != total or "" in ids:
        raise RuntimeError("Incomplete/duplicate list; preserve previous snapshot")
    jobs = []
    with ThreadPoolExecutor(max_workers=4) as pool:
        futures = {pool.submit(fetch_job_info, str(item["id"]), 3, 20): item for item in items}
        for future in as_completed(futures):
            item = futures[future]
            detail = future.result()
            if str(detail.get("id")) != str(item["id"]) or not detail.get("title") or not detail.get("description") or not detail.get("jobRequire"):
                raise RuntimeError("Invalid detail for " + str(item["id"]))
            jobs.append(normalize_job(item, detail))
            if len(jobs) % 25 == 0:
                log(f"Validated {len(jobs)}/{total}")
    return sorted(jobs, key=lambda x: x["id"])

def run():
    from datetime import timezone, timedelta
    import os
    parser = argparse.ArgumentParser()
    parser.add_argument("--output", required=True)
    parser.add_argument("--previous")
    args = parser.parse_args()
    previous = None
    if args.previous:
        previous = json.loads(Path(args.previous).read_text(encoding="utf-8"))
        if previous.get("status") != "complete":
            raise RuntimeError("Previous snapshot is not complete")
    jobs = collect()
    old = {j["id"]: j for j in previous["jobs"]} if previous else {}
    new = {j["id"]: j for j in jobs}
    fields = ["title", "addresses", "description", "job_require", "addition", "hurry", "project_name"]
    changes = {"baseline": previous is None, "added": [], "removed": [], "changed": []}
    if previous:
        changes["added"] = sorted(set(new) - set(old))
        changes["removed"] = [{"id": i, "title": old[i]["title"]} for i in sorted(set(old) - set(new))]
        changes["changed"] = [{"id": i, "fields": [f for f in fields if old[i].get(f) != new[i].get(f)]} for i in sorted(set(new) & set(old)) if any(old[i].get(f) != new[i].get(f) for f in fields)]
    snapshot = {"schema_version": 1, "owner": "game-jobs-weekly-sync", "status": "complete",
        "generated_at": datetime.now(timezone(timedelta(hours=8))).isoformat(),
        "source": "miHoYo public social recruitment APIs", "total": len(jobs),
        "previous_generated_at": previous.get("generated_at") if previous else None,
        "changes": changes, "jobs": jobs}
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    staging = output.with_suffix(output.suffix + ".tmp")
    try:
        staging.write_text(json.dumps(snapshot, ensure_ascii=False, indent=2), encoding="utf-8")
        check = json.loads(staging.read_text(encoding="utf-8"))
        assert check["total"] == len(check["jobs"]) == len(new)
        os.replace(staging, output)
    finally:
        staging.unlink(missing_ok=True)
    log(json.dumps({"status": "complete", "total": len(jobs), "bytes": output.stat().st_size, "output": str(output)}, ensure_ascii=False))

if __name__ == "__main__":
    run()
