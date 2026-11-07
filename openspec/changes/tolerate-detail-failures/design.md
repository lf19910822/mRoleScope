# Design

## Detail outcome boundary

List pagination remains a batch-level contract: an empty, inconsistent, duplicate, or otherwise invalid list prevents publication because the system cannot know the observation scope. After the list is validated, each requested detail is processed independently. Transport errors, non-success responses, unknown response shapes, ID mismatches, and missing required fields all become a failed detail outcome for that source job ID.

The collector writes successful detail responses and a sorted `failed_source_job_ids` list into a `complete` manifest. `detail_succeeded + detail_failed` equals the validated list size. A batch with at least one successful detail is publishable; a batch with zero successful details remains a failed run to avoid manufacturing an all-zero observation.

## Publication and retry semantics

The Go parser accepts the partition of list IDs into successful detail files and failed IDs. The publication transaction persists valid observations and the failure count. Failed IDs are excluded from missing-period rechecks and closure, so an existing job is not marked absent because its detail failed. A new failed ID has no observation row yet. The next list-based run requests it again without a separate retry queue.

The manifest and run audit retain counts rather than raw response bodies. Existing atomicity remains for the usable portion: archive verification and the database publication transaction must still both succeed.
