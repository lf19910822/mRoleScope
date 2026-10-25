package jdanalysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	AIOutputAuditPolicyVersionV1       = "cvm-ai-output-audit-v1"
	aiOutputAuditMaxResponseBytes      = 1 << 20
	aiOutputAuditMetadataReservation   = 4 << 10
	aiOutputAuditReservationBytes      = aiOutputAuditMaxResponseBytes + aiOutputAuditMetadataReservation
	aiOutputAuditDefaultCapacityBytes  = 64 << 20
	aiOutputAuditDefaultRetention      = 7 * 24 * time.Hour
	aiOutputAuditMetadataFileExtension = ".meta.json"
)

func aiOutputAuditInputContractComponents() []string {
	policy := defaultAIOutputAuditPolicy()
	return []string{
		AIOutputAuditPolicyVersionV1,
		"retention=" + aiOutputAuditRetentionContractValue(policy.Retention),
		"capacity_bytes=" + strconv.FormatInt(policy.CapacityBytes, 10),
		"response_reservation_bytes=" + strconv.FormatInt(aiOutputAuditReservationBytes, 10),
	}
}

func aiOutputAuditRetentionContractValue(retention time.Duration) string {
	if retention%time.Hour == 0 {
		return strconv.FormatInt(int64(retention/time.Hour), 10) + "h"
	}
	return retention.String()
}

// AIOutputAuditPolicySHA256 freezes the CVM-only retention and capacity
// contract without exposing any artifact path or model content.
func AIOutputAuditPolicySHA256() string {
	return sha256Hex(strings.Join(aiOutputAuditInputContractComponents(), "|"))
}

var (
	ErrAIOutputAuditCapacity = errors.New("AI output audit capacity is unavailable")
	aiOutputAuditUUIDPattern = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[1-5][a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$`)
)

// AIOutputAuditIdentity binds one raw response to its durable, non-text AI
// ledger identity. Values are validated before they become a filesystem path.
type AIOutputAuditIdentity struct {
	ConfigurationID      string
	WorkItemID           string
	AttemptNumber        int
	BodyContentVersionID string
}

// AIOutputAuditArtifact contains only local paths and a digest. Its response
// bytes never cross this seam into PostgreSQL, ordinary logging, or status.
type AIOutputAuditArtifact struct {
	CreatedAt      time.Time
	MetadataPath   string
	ResponsePath   string
	ResponseSHA256 string
}

// AIOutputAuditStatus contains only aggregate artifact storage facts. It
// deliberately omits paths, identities, SHA values, and all model text.
type AIOutputAuditStatus struct {
	AvailableBytes        int64
	CapacityBytes         int64
	CapturedArtifactCount int
	UsedBytes             int64
}

type aiOutputAuditPolicy struct {
	CapacityBytes int64
	Now           func() time.Time
	Retention     time.Duration
}

func defaultAIOutputAuditPolicy() aiOutputAuditPolicy {
	return aiOutputAuditPolicy{
		CapacityBytes: aiOutputAuditDefaultCapacityBytes,
		Retention:     aiOutputAuditDefaultRetention,
	}
}

type aiOutputAuditStore struct {
	mu           sync.Mutex
	policy       aiOutputAuditPolicy
	reservations map[*AIOutputAuditReservation]struct{}
	root         string
}

// AIOutputAuditReservation reserves a bounded response slot before the
// provider request. It is in-memory because the root-owned lifecycle helper
// globally serializes AI operations; a restarted worker conservatively starts
// with no provider response to recover.
type AIOutputAuditReservation struct {
	active bool
	store  *aiOutputAuditStore
}

type aiOutputAuditMetadata struct {
	ArtifactVersion      string    `json:"artifact_version"`
	BodyContentVersionID string    `json:"body_content_version_id"`
	ConfigurationID      string    `json:"configuration_id"`
	CreatedAt            time.Time `json:"created_at"`
	OutputSHA256         string    `json:"output_sha256"`
	PolicyVersion        string    `json:"policy_version"`
	AttemptNumber        int       `json:"attempt_number"`
	WorkItemID           string    `json:"work_item_id"`
}

func newAIOutputAuditStore(root string, policy aiOutputAuditPolicy) *aiOutputAuditStore {
	defaults := defaultAIOutputAuditPolicy()
	if policy.CapacityBytes == 0 {
		policy.CapacityBytes = defaults.CapacityBytes
	}
	if policy.Retention == 0 {
		policy.Retention = defaults.Retention
	}
	if policy.Now == nil {
		policy.Now = time.Now
	}
	return &aiOutputAuditStore{
		policy:       policy,
		reservations: make(map[*AIOutputAuditReservation]struct{}),
		root:         root,
	}
}

// NewAIOutputAuditStore creates the fixed CVM root-only artifact store.
func NewAIOutputAuditStore(root string) *aiOutputAuditStore {
	return newAIOutputAuditStore(root, aiOutputAuditPolicy{})
}

// Reserve removes expired artifacts and guarantees space for the largest
// provider response before an external request may be issued.
func (store *aiOutputAuditStore) Reserve() (*AIOutputAuditReservation, error) {
	if store == nil {
		return nil, errors.New("AI output audit store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.ensureRootLocked(); err != nil {
		return nil, err
	}
	if err := store.cleanupExpiredLocked(); err != nil {
		return nil, err
	}
	usedBytes, err := store.usedBytesLocked()
	if err != nil {
		return nil, err
	}
	reservedBytes := int64(len(store.reservations)) * aiOutputAuditReservationBytes
	if store.policy.CapacityBytes < aiOutputAuditReservationBytes || usedBytes > store.policy.CapacityBytes-aiOutputAuditReservationBytes-reservedBytes {
		return nil, ErrAIOutputAuditCapacity
	}
	reservation := &AIOutputAuditReservation{active: true, store: store}
	store.reservations[reservation] = struct{}{}
	return reservation, nil
}

// Release returns an unused pre-send reservation after a provider failure.
func (reservation *AIOutputAuditReservation) Release() error {
	if reservation == nil || reservation.store == nil {
		return errors.New("AI output audit reservation is unavailable")
	}
	store := reservation.store
	store.mu.Lock()
	defer store.mu.Unlock()
	if !reservation.active {
		return errors.New("AI output audit reservation is inactive")
	}
	delete(store.reservations, reservation)
	reservation.active = false
	return nil
}

// Capture atomically saves byte-identical provider output and non-text
// metadata, then consumes the reservation. It refuses oversized responses.
func (reservation *AIOutputAuditReservation) Capture(identity AIOutputAuditIdentity, response []byte) (AIOutputAuditArtifact, error) {
	if reservation == nil || reservation.store == nil {
		return AIOutputAuditArtifact{}, errors.New("AI output audit reservation is unavailable")
	}
	if !validAIOutputAuditIdentity(identity) || len(response) == 0 || len(response) > aiOutputAuditMaxResponseBytes {
		return AIOutputAuditArtifact{}, errors.New("AI output audit artifact is invalid")
	}
	store := reservation.store
	store.mu.Lock()
	defer store.mu.Unlock()
	if !reservation.active {
		return AIOutputAuditArtifact{}, errors.New("AI output audit reservation is inactive")
	}
	if err := store.ensureRootLocked(); err != nil {
		return AIOutputAuditArtifact{}, err
	}
	createdAt := store.policy.Now().UTC()
	directory := filepath.Join(store.root, identity.ConfigurationID, identity.WorkItemID)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return AIOutputAuditArtifact{}, err
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return AIOutputAuditArtifact{}, err
	}
	baseName := fmt.Sprintf("attempt-%02d", identity.AttemptNumber)
	responsePath := filepath.Join(directory, baseName+".response.json")
	metadataPath := filepath.Join(directory, baseName+aiOutputAuditMetadataFileExtension)
	if _, err := os.Lstat(responsePath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return AIOutputAuditArtifact{}, errors.New("AI output audit artifact already exists")
	}
	if _, err := os.Lstat(metadataPath); err == nil || !errors.Is(err, os.ErrNotExist) {
		return AIOutputAuditArtifact{}, errors.New("AI output audit metadata already exists")
	}
	sum := sha256.Sum256(response)
	outputSHA256 := hex.EncodeToString(sum[:])
	metadataBytes, err := json.Marshal(aiOutputAuditMetadata{
		ArtifactVersion:      "ai-output-audit-artifact-v1",
		BodyContentVersionID: identity.BodyContentVersionID,
		ConfigurationID:      identity.ConfigurationID,
		CreatedAt:            createdAt,
		OutputSHA256:         outputSHA256,
		PolicyVersion:        AIOutputAuditPolicyVersionV1,
		AttemptNumber:        identity.AttemptNumber,
		WorkItemID:           identity.WorkItemID,
	})
	if err != nil || len(metadataBytes) > aiOutputAuditMetadataReservation {
		return AIOutputAuditArtifact{}, errors.New("AI output audit metadata is invalid")
	}
	if err := atomicWriteAIOutputAuditFile(responsePath, response); err != nil {
		return AIOutputAuditArtifact{}, err
	}
	if err := atomicWriteAIOutputAuditFile(metadataPath, metadataBytes); err != nil {
		_ = os.Remove(responsePath)
		return AIOutputAuditArtifact{}, err
	}
	delete(store.reservations, reservation)
	reservation.active = false
	return AIOutputAuditArtifact{
		CreatedAt:      createdAt,
		MetadataPath:   metadataPath,
		ResponsePath:   responsePath,
		ResponseSHA256: outputSHA256,
	}, nil
}

// Load returns an integrity-checked captured response for the fixed terminal
// audit session. It validates every path component from the durable identity
// and never accepts a caller-provided filename.
func (store *aiOutputAuditStore) Load(identity AIOutputAuditIdentity) (AIOutputAuditArtifact, []byte, error) {
	if store == nil || !validAIOutputAuditIdentity(identity) {
		return AIOutputAuditArtifact{}, nil, errors.New("AI output audit artifact is invalid")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	basePath := filepath.Join(store.root, identity.ConfigurationID, identity.WorkItemID, fmt.Sprintf("attempt-%02d", identity.AttemptNumber))
	responsePath := basePath + ".response.json"
	metadataPath := basePath + aiOutputAuditMetadataFileExtension
	for _, path := range []string{responsePath, metadataPath} {
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return AIOutputAuditArtifact{}, nil, errors.New("AI output audit artifact is unavailable")
		}
	}
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return AIOutputAuditArtifact{}, nil, err
	}
	var metadata aiOutputAuditMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil || !validAIOutputAuditMetadata(metadata) ||
		metadata.ConfigurationID != identity.ConfigurationID || metadata.WorkItemID != identity.WorkItemID ||
		metadata.AttemptNumber != identity.AttemptNumber || metadata.BodyContentVersionID != identity.BodyContentVersionID {
		return AIOutputAuditArtifact{}, nil, errors.New("AI output audit metadata is invalid")
	}
	response, err := os.ReadFile(responsePath)
	if err != nil || len(response) == 0 || len(response) > aiOutputAuditMaxResponseBytes {
		return AIOutputAuditArtifact{}, nil, errors.New("AI output audit response is invalid")
	}
	sum := sha256.Sum256(response)
	if actual := hex.EncodeToString(sum[:]); actual != metadata.OutputSHA256 {
		return AIOutputAuditArtifact{}, nil, errors.New("AI output audit response integrity mismatch")
	}
	return AIOutputAuditArtifact{
		CreatedAt:      metadata.CreatedAt,
		MetadataPath:   metadataPath,
		ResponsePath:   responsePath,
		ResponseSHA256: metadata.OutputSHA256,
	}, response, nil
}

// Has verifies only fixed-path metadata and file attributes for one durable
// identity. It does not read response bytes, so safe status code can count
// auditable failures without receiving model output.
func (store *aiOutputAuditStore) Has(identity AIOutputAuditIdentity) (bool, error) {
	if store == nil || !validAIOutputAuditIdentity(identity) {
		return false, errors.New("AI output audit artifact is invalid")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	basePath := filepath.Join(store.root, identity.ConfigurationID, identity.WorkItemID, fmt.Sprintf("attempt-%02d", identity.AttemptNumber))
	responsePath := basePath + ".response.json"
	metadataPath := basePath + aiOutputAuditMetadataFileExtension
	for _, path := range []string{responsePath, metadataPath} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
			return false, errors.New("AI output audit artifact is unavailable")
		}
	}
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return false, err
	}
	var metadata aiOutputAuditMetadata
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil || !validAIOutputAuditMetadata(metadata) ||
		metadata.ConfigurationID != identity.ConfigurationID || metadata.WorkItemID != identity.WorkItemID ||
		metadata.AttemptNumber != identity.AttemptNumber || metadata.BodyContentVersionID != identity.BodyContentVersionID {
		return false, errors.New("AI output audit metadata is invalid")
	}
	return true, nil
}

// Status calculates safe aggregate storage facts from metadata and file
// attributes. It never opens a response file or prints storage paths.
func (store *aiOutputAuditStore) Status() (AIOutputAuditStatus, error) {
	if store == nil || store.root == "" || store.policy.CapacityBytes < aiOutputAuditReservationBytes || store.policy.Retention <= 0 {
		return AIOutputAuditStatus{}, errors.New("AI output audit configuration is invalid")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	info, err := os.Stat(store.root)
	if err != nil || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return AIOutputAuditStatus{}, errors.New("AI output audit root is unavailable")
	}
	metadataPaths, err := store.metadataPathsLocked()
	if err != nil {
		return AIOutputAuditStatus{}, err
	}
	captured := 0
	for _, metadataPath := range metadataPaths {
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return AIOutputAuditStatus{}, err
		}
		var metadata aiOutputAuditMetadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil || !validAIOutputAuditMetadata(metadata) {
			return AIOutputAuditStatus{}, errors.New("AI output audit metadata is invalid")
		}
		responseInfo, err := os.Lstat(responsePathForMetadata(metadataPath))
		if err != nil || !responseInfo.Mode().IsRegular() || responseInfo.Mode().Perm() != 0o600 {
			return AIOutputAuditStatus{}, errors.New("AI output audit artifact is unavailable")
		}
		captured++
	}
	usedBytes, err := store.usedBytesLocked()
	if err != nil {
		return AIOutputAuditStatus{}, err
	}
	availableBytes := store.policy.CapacityBytes - usedBytes
	if availableBytes < 0 {
		availableBytes = 0
	}
	return AIOutputAuditStatus{
		AvailableBytes:        availableBytes,
		CapacityBytes:         store.policy.CapacityBytes,
		CapturedArtifactCount: captured,
		UsedBytes:             usedBytes,
	}, nil
}

func (store *aiOutputAuditStore) ensureRootLocked() error {
	if store.root == "" || store.policy.CapacityBytes < aiOutputAuditReservationBytes || store.policy.Retention <= 0 {
		return errors.New("AI output audit configuration is invalid")
	}
	if err := os.MkdirAll(store.root, 0o700); err != nil {
		return err
	}
	return os.Chmod(store.root, 0o700)
}

func (store *aiOutputAuditStore) cleanupExpiredLocked() error {
	if err := store.cleanupIncompleteArtifactPairsLocked(); err != nil {
		return err
	}
	metadataPaths, err := store.metadataPathsLocked()
	if err != nil {
		return err
	}
	now := store.policy.Now().UTC()
	for _, metadataPath := range metadataPaths {
		metadataBytes, err := os.ReadFile(metadataPath)
		if err != nil {
			return err
		}
		var metadata aiOutputAuditMetadata
		if err := json.Unmarshal(metadataBytes, &metadata); err != nil || !validAIOutputAuditMetadata(metadata) {
			return errors.New("AI output audit metadata is invalid")
		}
		if metadata.CreatedAt.Add(store.policy.Retention).After(now) {
			continue
		}
		responsePath := responsePathForMetadata(metadataPath)
		if err := os.Remove(responsePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.Remove(metadataPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// cleanupIncompleteArtifactPairsLocked removes a response or sidecar that was
// left alone by a crash or a failed second atomic write. A pair becomes an
// auditable artifact only after both fixed files exist; removing incomplete
// pairs before every reservation keeps a raw orphan inside the same bounded
// retention lifecycle without ever resending the provider request.
func (store *aiOutputAuditStore) cleanupIncompleteArtifactPairsLocked() error {
	return filepath.WalkDir(store.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		var pairedPath string
		switch {
		case strings.HasSuffix(path, ".response.json"):
			pairedPath = strings.TrimSuffix(path, ".response.json") + aiOutputAuditMetadataFileExtension
		case strings.HasSuffix(path, aiOutputAuditMetadataFileExtension):
			pairedPath = responsePathForMetadata(path)
		default:
			return nil
		}
		pairedInfo, pairedErr := os.Lstat(pairedPath)
		if pairedErr == nil && pairedInfo.Mode().IsRegular() {
			return nil
		}
		if pairedErr != nil && !errors.Is(pairedErr, os.ErrNotExist) {
			return pairedErr
		}
		return os.Remove(path)
	})
}

func (store *aiOutputAuditStore) metadataPathsLocked() ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(store.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() || filepath.Ext(path) != ".json" || len(path) < len(aiOutputAuditMetadataFileExtension) || path[len(path)-len(aiOutputAuditMetadataFileExtension):] != aiOutputAuditMetadataFileExtension {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

func (store *aiOutputAuditStore) usedBytesLocked() (int64, error) {
	var total int64
	err := filepath.WalkDir(store.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		total += info.Size()
		return nil
	})
	return total, err
}

func atomicWriteAIOutputAuditFile(path string, content []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".ai-output-audit-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func responsePathForMetadata(metadataPath string) string {
	return metadataPath[:len(metadataPath)-len(aiOutputAuditMetadataFileExtension)] + ".response.json"
}

func validAIOutputAuditIdentity(identity AIOutputAuditIdentity) bool {
	return aiOutputAuditUUIDPattern.MatchString(identity.ConfigurationID) &&
		aiOutputAuditUUIDPattern.MatchString(identity.WorkItemID) &&
		aiOutputAuditUUIDPattern.MatchString(identity.BodyContentVersionID) &&
		identity.AttemptNumber >= 1 && identity.AttemptNumber <= aiModelMaximumAttempts
}

func validAIOutputAuditMetadata(metadata aiOutputAuditMetadata) bool {
	return metadata.ArtifactVersion == "ai-output-audit-artifact-v1" &&
		metadata.PolicyVersion == AIOutputAuditPolicyVersionV1 &&
		validAIOutputAuditIdentity(AIOutputAuditIdentity{
			ConfigurationID:      metadata.ConfigurationID,
			WorkItemID:           metadata.WorkItemID,
			AttemptNumber:        metadata.AttemptNumber,
			BodyContentVersionID: metadata.BodyContentVersionID,
		}) && metadata.CreatedAt.Location() == time.UTC &&
		len(metadata.OutputSHA256) == sha256.Size*2
}
