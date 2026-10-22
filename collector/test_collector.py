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
