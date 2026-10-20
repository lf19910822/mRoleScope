# Python collector

`mihoyo_collector.py` is a no-third-party-dependency adaptation of the existing weekly synchronizer in `Docs/同步任务/`. It keeps the documented public endpoints and four-detail-worker default, but changes the output boundary:

- raw list and detail API responses are retained separately;
- a `schema_version=1` manifest records scope, timing, counts and SHA-256 values;
- an incomplete response yields `status=failed`, never a publishable partial task;
- the collector does not calculate business aggregates, diffs or publication state.

Run the offline contract fixture without contacting the public API:

```bash
python3 -m unittest collector.test_collector
python3 collector/mihoyo_collector.py \
  --run-id local-fixture \
  --fixture collector/fixtures/complete_collection.json \
  --output-dir /tmp/mrolescope-fixture
```

Production collection must be started by the Go worker after its durable run record and lease exist. Do not schedule this script directly in parallel with the legacy weekly task until the old/new boundary has been explicitly switched.
