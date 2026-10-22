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

## Memory-only public API probe

Before a real collection pipeline and its retention policy are enabled, verify
the two public endpoints without creating a task directory or retaining any
response. The probe requests one list item and that item's detail in memory,
validates the response shape, and prints only status metadata (never a job ID,
title, description, or raw response):

```bash
python3 -B collector/mihoyo_collector.py --probe --retries 1 --timeout-seconds 12
```

`--probe` cannot be combined with `--output-dir`, `--fixture`, or `--run-id`.
It is a reachability and contract check only; it is not a successful collection
run and does not create an archive, manifest, database record, or publication.

Production collection must be started by the Go worker after its durable run record and lease exist. Do not schedule this script directly in parallel with the legacy weekly task until the old/new boundary has been explicitly switched.
