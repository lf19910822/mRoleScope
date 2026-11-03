package main

import (
	"context"
	"errors"
	"testing"

	"mrolescope/internal/jdanalysis"
	"mrolescope/internal/jdserving"
)

type v03ReleaseStub struct {
	summary jdanalysis.ReleaseSummary
	err     error
	calls   int
}

func (stub *v03ReleaseStub) ReleaseV03Upgrade(context.Context, string) (jdanalysis.ReleaseSummary, error) {
	stub.calls++
	return stub.summary, stub.err
}

type servingProjectionStub struct {
	summary jdserving.ReconciliationSummary
	err     error
	calls   int
	closed  bool
}

func (stub *servingProjectionStub) ReconcilePublishedInventory(context.Context) (jdserving.ReconciliationSummary, error) {
	stub.calls++
	return stub.summary, stub.err
}

func (stub *servingProjectionStub) Close() {
	stub.closed = true
}

func TestPublishV03UpgradeReconcilesServingAfterEverySuccessfulRelease(t *testing.T) {
	for name, releaseSummary := range map[string]jdanalysis.ReleaseSummary{
		"first release":  {Published: true},
		"reused release": {Published: true, Reused: true},
	} {
		t.Run(name, func(t *testing.T) {
			release := &v03ReleaseStub{summary: releaseSummary}
			serving := &servingProjectionStub{summary: jdserving.ReconciliationSummary{ReconciledRuns: 3}}
			factoryCalls := 0

			gotRelease, gotServing, err := publishV03UpgradeAndReconcile(
				context.Background(), release, "postgres://synthetic", "reviewer",
				func(context.Context, string) (servingInventoryReconciler, error) {
					factoryCalls++
					return serving, nil
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if gotRelease != releaseSummary || gotServing != serving.summary || release.calls != 1 ||
				factoryCalls != 1 || serving.calls != 1 || !serving.closed {
				t.Fatalf("v0.3 publish reconciliation = release:%#v serving:%#v releaseCalls:%d factoryCalls:%d servingCalls:%d closed:%t",
					gotRelease, gotServing, release.calls, factoryCalls, serving.calls, serving.closed)
			}
		})
	}
}

func TestPublishV03UpgradeDoesNotReconcileWhenReleaseFails(t *testing.T) {
	release := &v03ReleaseStub{err: errors.New("release gate rejected")}
	factoryCalls := 0
	_, _, err := publishV03UpgradeAndReconcile(
		context.Background(), release, "postgres://synthetic", "reviewer",
		func(context.Context, string) (servingInventoryReconciler, error) {
			factoryCalls++
			return &servingProjectionStub{}, nil
		},
	)
	if err == nil || release.calls != 1 || factoryCalls != 0 {
		t.Fatalf("failed v0.3 release attempted serving reconciliation: err=%v releaseCalls=%d factoryCalls=%d", err, release.calls, factoryCalls)
	}
}

func TestPublishV03UpgradeReturnsRetryableServingFailureAfterCommittedRelease(t *testing.T) {
	release := &v03ReleaseStub{summary: jdanalysis.ReleaseSummary{Published: true}}
	serving := &servingProjectionStub{err: errors.New("serving unavailable")}
	gotRelease, _, err := publishV03UpgradeAndReconcile(
		context.Background(), release, "postgres://synthetic", "reviewer",
		func(context.Context, string) (servingInventoryReconciler, error) {
			return serving, nil
		},
	)
	if err == nil || gotRelease != release.summary || release.calls != 1 || serving.calls != 1 || !serving.closed {
		t.Fatalf("post-release serving failure = release:%#v err:%v releaseCalls:%d servingCalls:%d closed:%t",
			gotRelease, err, release.calls, serving.calls, serving.closed)
	}
}
