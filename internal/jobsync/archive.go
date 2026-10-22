package jobsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"reflect"
	"strings"

	"mrolescope/internal/collector"
)

const checksumMetadataKey = "sha256"

type ObjectInfo struct {
	Size     int64
	Metadata map[string]string
}

type ObjectStore interface {
	Put(context.Context, string, io.Reader, int64, map[string]string) error
	Head(context.Context, string) (ObjectInfo, error)
	URI(string) string
}

// Archiver uploads exactly the manifest and entries the collector already
// hash-validated. It has no delete capability: failed DB publication leaves a
// safe, unreferenced evidence prefix for later human review.
type Archiver struct {
	Store  ObjectStore
	Prefix string
}

func (archiver Archiver) Archive(ctx context.Context, root string, scope Scope, dataDate string, manifest collector.Manifest) (ArchiveResult, error) {
	if archiver.Store == nil {
		return ArchiveResult{}, fmt.Errorf("object store is required")
	}
	if err := manifest.Validate(); err != nil {
		return ArchiveResult{}, fmt.Errorf("validate manifest before archive: %w", err)
	}
	// The file on disk is archived as evidence. Verify it is the exact
	// manifest that the caller validated, rather than merely uploading a file
	// named manifest.json after the source files have been checked.
	diskManifest, err := collector.ReadManifest(path.Join(root, "manifest.json"))
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("read manifest for archive: %w", err)
	}
	if !reflect.DeepEqual(diskManifest, manifest) {
		return ArchiveResult{}, fmt.Errorf("manifest changed after validation")
	}
	if err := collector.ValidateFiles(root, diskManifest); err != nil {
		return ArchiveResult{}, fmt.Errorf("validate archive inputs: %w", err)
	}
	prefix := strings.Trim(archiver.Prefix, "/")
	if prefix == "" {
		return ArchiveResult{}, fmt.Errorf("archive prefix is required")
	}
	baseKey := path.Join(prefix, scope.SourceCompany, scope.FilterSHA256, dataDate, manifest.RunID)
	entries := append([]collector.File(nil), diskManifest.Files...)
	manifestContents, err := os.ReadFile(path.Join(root, "manifest.json"))
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("read manifest for archive: %w", err)
	}
	manifestSum := sha256.Sum256(manifestContents)
	entries = append(entries, collector.File{Path: "manifest.json", SHA256: hex.EncodeToString(manifestSum[:]), Bytes: int64(len(manifestContents))})
	for _, entry := range entries {
		if err := archiveFile(ctx, archiver.Store, path.Join(baseKey, entry.Path), path.Join(root, entry.Path), entry); err != nil {
			return ArchiveResult{}, err
		}
	}
	return ArchiveResult{
		URI:            archiver.Store.URI(baseKey),
		ManifestSHA256: hex.EncodeToString(manifestSum[:]),
	}, nil
}

func archiveFile(ctx context.Context, store ObjectStore, objectKey, filePath string, expected collector.File) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open archive input %s: %w", expected.Path, err)
	}
	defer file.Close()
	sum := sha256.New()
	reader := &countingReader{reader: io.TeeReader(file, sum)}
	if err := store.Put(ctx, objectKey, reader, expected.Bytes, map[string]string{checksumMetadataKey: expected.SHA256}); err != nil {
		return fmt.Errorf("upload %s: %w", expected.Path, err)
	}
	actualSHA256 := hex.EncodeToString(sum.Sum(nil))
	if reader.bytes != expected.Bytes || actualSHA256 != expected.SHA256 {
		return fmt.Errorf("archive input changed while uploading %s", expected.Path)
	}
	info, err := store.Head(ctx, objectKey)
	if err != nil {
		return fmt.Errorf("verify %s: %w", expected.Path, err)
	}
	if info.Size != expected.Bytes || info.Metadata[checksumMetadataKey] != expected.SHA256 {
		return fmt.Errorf("archive verification failed for %s", expected.Path)
	}
	return nil
}

type countingReader struct {
	reader io.Reader
	bytes  int64
}

func (reader *countingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.bytes += int64(count)
	return count, err
}
