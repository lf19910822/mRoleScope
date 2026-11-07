import hashlib
import io
import json
import tempfile
import unittest
from contextlib import redirect_stdout
from email.message import Message
from pathlib import Path
from urllib import error
from unittest.mock import patch

from collector import mihoyo_collector


class CollectorContractTests(unittest.TestCase):
    def test_detail_payload_matches_existing_sync_script_contract(self):
        self.assertEqual(
            {"id": "8771", "channelDetailIds": [1], "hireType": 0},
            mihoyo_collector.detail_payload("8771"),
        )

    def test_retry_after_is_preferred_for_rate_limits(self):
        headers = Message()
        headers["Retry-After"] = "12"
        response_error = error.HTTPError("https://example.test", 429, "Too Many Requests", headers, None)
        self.assertEqual(12.0, mihoyo_collector.retry_delay_seconds(1, response_error))

    def test_collector_keeps_existing_browser_user_agent(self):
        self.assertEqual(
            "Mozilla/5.0 (Windows NT 10.0; Win64; x64) "
            "AppleWebKit/537.36 (KHTML, like Gecko) "
            "Chrome/120.0.0.0 Safari/537.36",
            mihoyo_collector.HEADERS["User-Agent"],
        )

    def test_complete_fixture_writes_raw_files_and_manifest(self):
        fixture = Path(__file__).parent / "fixtures" / "complete_collection.json"
        with tempfile.TemporaryDirectory() as temporary_directory:
            output = Path(temporary_directory) / "run"
            result = mihoyo_collector.run(
                ["--output-dir", str(output), "--run-id", "fixture-run", "--fixture", str(fixture)]
            )
            self.assertEqual(0, result)
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual("complete", manifest["status"])
            self.assertEqual("fixture-run", manifest["run_id"])
            self.assertEqual(1, manifest["counts"]["detail_succeeded"])
            self.assertEqual(2, len(manifest["files"]))
            for file in manifest["files"]:
                contents = (output / file["path"]).read_bytes()
                self.assertEqual(file["bytes"], len(contents))
                self.assertEqual(file["sha256"], hashlib.sha256(contents).hexdigest())

    def test_partial_detail_failure_keeps_valid_jobs_and_records_failed_id(self):
        fixture = {
            "list_pages": [
                {
                    "success": True,
                    "data": {
                        "total": 2,
                        "list": [
                            {"id": "valid-job", "title": "Valid"},
                            {"id": "malformed-job", "title": "Malformed"},
                        ],
                    },
                }
            ],
            "details": {
                "valid-job": {
                    "success": True,
                    "data": {
                        "id": "valid-job",
                        "title": "Valid",
                        "description": "body",
                        "jobRequire": "requirements",
                    },
                },
                "malformed-job": {
                    "success": True,
                    "data": {
                        "id": "malformed-job",
                        "title": "Malformed",
                        "description": "",
                        "jobRequire": "requirements",
                    },
                },
            },
        }
        with tempfile.TemporaryDirectory() as temporary_directory:
            fixture_path = Path(temporary_directory) / "partial.json"
            fixture_path.write_text(json.dumps(fixture), encoding="utf-8")
            output = Path(temporary_directory) / "run"

            result = mihoyo_collector.run(
                ["--output-dir", str(output), "--run-id", "partial-run", "--fixture", str(fixture_path)]
            )

            self.assertEqual(0, result)
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(1, manifest["counts"]["detail_succeeded"])
            self.assertEqual(1, manifest["counts"]["detail_failed"])
            self.assertEqual(["malformed-job"], manifest["failed_source_job_ids"])
            self.assertTrue((output / "raw/details/valid-job.json").exists())
            self.assertFalse((output / "raw/details/malformed-job.json").exists())

    def test_transport_and_shape_failures_are_isolated_per_detail(self):
        items = [{"id": "valid-job"}, {"id": "transport-job"}, {"id": "mismatch-job"}]

        def fake_post(url, payload, retries, timeout_seconds):
            self.assertEqual(mihoyo_collector.DETAIL_API, url)
            self.assertEqual((3, 20), (retries, timeout_seconds))
            if payload["id"] == "transport-job":
                raise mihoyo_collector.CollectionError("temporary upstream failure")
            if payload["id"] == "mismatch-job":
                return {
                    "success": True,
                    "data": {"id": "other-job", "title": "Wrong", "description": "body", "jobRequire": "req"},
                }
            return {
                "success": True,
                "data": {"id": "valid-job", "title": "Valid", "description": "body", "jobRequire": "req"},
            }

        with patch.object(mihoyo_collector, "post_json", side_effect=fake_post):
            details, failed_ids = mihoyo_collector.collect_detail_responses(items, workers=2, retries=3, timeout_seconds=20)

        self.assertEqual(["valid-job"], sorted(details))
        self.assertEqual(["mismatch-job", "transport-job"], failed_ids)

    def test_all_detail_failure_kinds_are_isolated_by_their_own_id(self):
        items = [
            {"id": "usable-arbitrary-id"},
            {"id": "business-failure-arbitrary-id"},
            {"id": "shape-failure-arbitrary-id"},
            {"id": "mismatch-arbitrary-id"},
            {"id": "empty-body-arbitrary-id"},
        ]

        def fake_post(url, payload, retries, timeout_seconds):
            self.assertEqual(mihoyo_collector.DETAIL_API, url)
            source_job_id = payload["id"]
            if source_job_id == "business-failure-arbitrary-id":
                return {"success": False, "data": None}
            if source_job_id == "shape-failure-arbitrary-id":
                return {"success": True, "data": []}
            if source_job_id == "mismatch-arbitrary-id":
                return {
                    "success": True,
                    "data": {
                        "id": "different-arbitrary-id",
                        "title": "Wrong",
                        "description": "body",
                        "jobRequire": "requirements",
                    },
                }
            if source_job_id == "empty-body-arbitrary-id":
                return {
                    "success": True,
                    "data": {
                        "id": source_job_id,
                        "title": "Title",
                        "description": "<p> </p>",
                        "jobRequire": "requirements",
                    },
                }
            return {
                "success": True,
                "data": {
                    "id": source_job_id,
                    "title": "Title",
                    "description": "body",
                    "jobRequire": "requirements",
                },
            }

        with patch.object(mihoyo_collector, "post_json", side_effect=fake_post):
            details, failed_ids = mihoyo_collector.collect_detail_responses(
                items, workers=3, retries=2, timeout_seconds=20
            )

        self.assertEqual(["usable-arbitrary-id"], sorted(details))
        self.assertEqual(
            [
                "business-failure-arbitrary-id",
                "empty-body-arbitrary-id",
                "mismatch-arbitrary-id",
                "shape-failure-arbitrary-id",
            ],
            failed_ids,
        )

    def test_fixture_non_object_detail_response_is_isolated(self):
        fixture = {
            "list_pages": [
                {
                    "success": True,
                    "data": {
                        "total": 2,
                        "list": [{"id": "fixture-valid-id"}, {"id": "fixture-shape-failure-id"}],
                    },
                }
            ],
            "details": {
                "fixture-valid-id": {
                    "success": True,
                    "data": {
                        "id": "fixture-valid-id",
                        "title": "Title",
                        "description": "body",
                        "jobRequire": "requirements",
                    },
                },
                "fixture-shape-failure-id": [],
            },
        }
        with tempfile.TemporaryDirectory() as temporary_directory:
            fixture_path = Path(temporary_directory) / "shape-failure.json"
            fixture_path.write_text(json.dumps(fixture), encoding="utf-8")
            output = Path(temporary_directory) / "run"

            result = mihoyo_collector.run(["--output-dir", str(output), "--fixture", str(fixture_path)])

            self.assertEqual(0, result)
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual(["fixture-shape-failure-id"], manifest["failed_source_job_ids"])

    def test_writer_rejects_incomplete_detail_outcome_partition(self):
        pages = [{"success": True, "data": {"total": 2, "list": [{"id": "listed-a"}, {"id": "listed-b"}]}}]
        details = {
            "listed-a": {
                "success": True,
                "data": {"id": "listed-a", "title": "A", "description": "body", "jobRequire": "req"},
            }
        }
        with tempfile.TemporaryDirectory() as temporary_directory:
            with self.assertRaises(mihoyo_collector.CollectionError):
                mihoyo_collector.write_task_directory(
                    Path(temporary_directory),
                    "partition-run",
                    "2026-11-07T00:00:00Z",
                    pages,
                    details,
                    retries=1,
                    timeout_seconds=1,
                    workers=1,
                )

    def test_all_detail_failures_remain_a_failed_run(self):
        fixture = {
            "list_pages": [{"success": True, "data": {"total": 1, "list": [{"id": "only-job"}]}}],
            "details": {
                "only-job": {
                    "success": True,
                    "data": {"id": "only-job", "title": "Only", "description": "", "jobRequire": ""},
                }
            },
        }
        with tempfile.TemporaryDirectory() as temporary_directory:
            fixture_path = Path(temporary_directory) / "all-failed.json"
            fixture_path.write_text(json.dumps(fixture), encoding="utf-8")
            output = Path(temporary_directory) / "run"

            result = mihoyo_collector.run(["--output-dir", str(output), "--fixture", str(fixture_path)])

            self.assertEqual(1, result)
            manifest = json.loads((output / "manifest.json").read_text(encoding="utf-8"))
            self.assertEqual("failed", manifest["status"])

    def test_rejects_nonempty_output_directory(self):
        fixture = Path(__file__).parent / "fixtures" / "complete_collection.json"
        with tempfile.TemporaryDirectory() as temporary_directory:
            output = Path(temporary_directory) / "run"
            output.mkdir()
            (output / "existing.txt").write_text("do not overwrite", encoding="utf-8")
            with self.assertRaises(mihoyo_collector.CollectionError):
                mihoyo_collector.run(["--output-dir", str(output), "--fixture", str(fixture)])

    def test_probe_checks_one_list_and_one_detail_without_writing_responses(self):
        requests = []

        def fake_post(url, payload, retries, timeout_seconds):
            requests.append((url, payload, retries, timeout_seconds))
            if url == mihoyo_collector.LIST_API:
                return {
                    "success": True,
                    "data": {"total": 159, "list": [{"id": "probe-job", "title": "must not print"}]},
                }
            if url == mihoyo_collector.DETAIL_API:
                return {
                    "success": True,
                    "data": {
                        "id": "probe-job",
                        "title": "must not print",
                        "description": "must not print",
                        "jobRequire": "must not print",
                    },
                }
            self.fail("unexpected URL: {0}".format(url))

        standard_output = io.StringIO()
        with patch.object(mihoyo_collector, "post_json", side_effect=fake_post), \
             patch.object(mihoyo_collector, "write_json", side_effect=AssertionError("probe must not write files")), \
             patch.object(mihoyo_collector, "write_task_directory", side_effect=AssertionError("probe must not write files")), \
             redirect_stdout(standard_output):
            result = mihoyo_collector.run(["--probe", "--retries", "1", "--timeout-seconds", "1"])

        self.assertEqual(0, result)
        self.assertEqual(
            [
                (mihoyo_collector.LIST_API, mihoyo_collector.list_payload(1, 1), 1, 1),
                (mihoyo_collector.DETAIL_API, mihoyo_collector.detail_payload("probe-job"), 1, 1),
            ],
            requests,
        )
        self.assertEqual(
            {
                "detail_validated": True,
                "list_reported_total": 159,
                "mode": "live-probe",
                "status": "ok",
            },
            json.loads(standard_output.getvalue()),
        )


if __name__ == "__main__":
    unittest.main()
