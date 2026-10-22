package jobsync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mrolescope/internal/collector"
)

type memoryObjectStore struct {
	objects    map[string]memoryObject
	putErr     error
	headMutate func(ObjectInfo) ObjectInfo
}

type memoryObject struct {
	contents []byte
	metadata map[string]string
}

func (store *memoryObjectStore) Put(_ context.Context, key string, body io.Reader, _ int64, metadata map[string]string) error {
	if store.putErr != nil {
		return store.putErr
	}
	contents, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	copyMetadata := make(map[string]string, len(metadata))
	for name, value := range metadata {
		copyMetadata[name] = value
	}
	store.objects[key] = memoryObject{contents: contents, metadata: copyMetadata}
	return nil
}

func (store *memoryObjectStore) Head(_ context.Context, key string) (ObjectInfo, error) {
	object, ok := store.objects[key]
	if !ok {
		return ObjectInfo{}, os.ErrNotExist
	}
	info := ObjectInfo{Size: int64(len(object.contents)), Metadata: object.metadata}
	if store.headMutate != nil {
		return store.headMutate(info), nil
	}
	return info, nil
}

func (store *memoryObjectStore) URI(key string) string { return "s3://test-bucket/" + key }

func TestArchiverUploadsAndVerifiesManifestAndFiles(t *testing.T) {
	root := t.TempDir()
	contents := []byte(`{"success":true}`)
	if err := os.MkdirAll(filepath.Join(root, "raw/details"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raw/details/1.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	now := time.Now().UTC()
	manifest := collector.Manifest{
		SchemaVersion: 1, Status: "complete", RunID: "run-1", StartedAt: now, FinishedAt: now,
		Source: collector.Source{Company: SourceCompanyMihoyo, Filter: MihoyoTechnicalScope().Filter, FilterSHA256: MihoyoTechnicalScope().FilterSHA256},
		Counts: collector.Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
		Files:  []collector.File{{Path: "raw/details/1.json", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(contents))}},
	}
	manifestContents, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), manifestContents, 0o600); err != nil {
		t.Fatal(err)
	}
	store := &memoryObjectStore{objects: map[string]memoryObject{}}
	result, err := (Archiver{Store: store, Prefix: "raw"}).Archive(context.Background(), root, MihoyoTechnicalScope(), "2026-10-22", manifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.ManifestSHA256 == "" || result.URI != "s3://test-bucket/raw/mihoyo/"+manifest.Source.FilterSHA256+"/2026-10-22/run-1" {
		t.Fatalf("unexpected archive result: %#v", result)
	}
	if got := store.objects["raw/mihoyo/"+manifest.Source.FilterSHA256+"/2026-10-22/run-1/raw/details/1.json"]; !bytes.Equal(got.contents, contents) || got.metadata[checksumMetadataKey] != hex.EncodeToString(sum[:]) {
		t.Fatalf("detail object not archived with checksum: %#v", got)
	}
	if _, exists := store.objects["raw/mihoyo/"+manifest.Source.FilterSHA256+"/2026-10-22/run-1/manifest.json"]; !exists {
		t.Fatal("manifest was not archived")
	}
}

func TestArchiverRejectsObjectStoreVerificationOrUploadFailure(t *testing.T) {
	root, manifest := writeArchiveFixture(t)
	for name, store := range map[string]*memoryObjectStore{
		"upload_failure": {objects: map[string]memoryObject{}, putErr: errors.New("object storage unavailable")},
		"metadata_mismatch": {
			objects: map[string]memoryObject{},
			headMutate: func(info ObjectInfo) ObjectInfo {
				info.Metadata = map[string]string{checksumMetadataKey: "wrong"}
				return info
			},
		},
		"size_mismatch": {
			objects: map[string]memoryObject{},
			headMutate: func(info ObjectInfo) ObjectInfo {
				info.Size++
				return info
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := (Archiver{Store: store, Prefix: "raw"}).Archive(context.Background(), root, MihoyoTechnicalScope(), "2026-10-22", manifest); err == nil {
				t.Fatal("expected archive verification failure")
			}
		})
	}
}

func TestArchiverRejectsManifestChangedAfterValidation(t *testing.T) {
	root, manifest := writeArchiveFixture(t)
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), []byte(`{"status":"tampered"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &memoryObjectStore{objects: map[string]memoryObject{}}
	if _, err := (Archiver{Store: store, Prefix: "raw"}).Archive(context.Background(), root, MihoyoTechnicalScope(), "2026-10-22", manifest); err == nil {
		t.Fatal("expected modified manifest to be rejected")
	}
}

func TestArchiverRejectsFileChangedAfterManifestCreation(t *testing.T) {
	root, manifest := writeArchiveFixture(t)
	if err := os.WriteFile(filepath.Join(root, "raw/details/1.json"), []byte(`{"success":false}`), 0o600); err != nil {
		t.Fatal(err)
	}
	store := &memoryObjectStore{objects: map[string]memoryObject{}}
	if _, err := (Archiver{Store: store, Prefix: "raw"}).Archive(context.Background(), root, MihoyoTechnicalScope(), "2026-10-22", manifest); err == nil {
		t.Fatal("expected modified evidence file to be rejected")
	}
}

func writeArchiveFixture(t *testing.T) (string, collector.Manifest) {
	t.Helper()
	root := t.TempDir()
	contents := []byte(`{"success":true}`)
	if err := os.MkdirAll(filepath.Join(root, "raw/details"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "raw/details/1.json"), contents, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(contents)
	now := time.Now().UTC()
	manifest := collector.Manifest{
		SchemaVersion: 1, Status: "complete", RunID: "run-1", StartedAt: now, FinishedAt: now,
		Source: collector.Source{Company: SourceCompanyMihoyo, Filter: MihoyoTechnicalScope().Filter, FilterSHA256: MihoyoTechnicalScope().FilterSHA256},
		Counts: collector.Counts{ListReportedTotal: 1, ListItems: 1, UniqueSourceJobID: 1, DetailSucceeded: 1},
		Files:  []collector.File{{Path: "raw/details/1.json", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(contents))}},
	}
	manifestContents, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), manifestContents, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, manifest
}
