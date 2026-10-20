import hashlib
import json
import tempfile
import unittest
from email.message import Message
from pathlib import Path
from urllib import error

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


if __name__ == "__main__":
    unittest.main()
