package jdanalysis

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	// AIInputTokenEstimatorVersionV1 identifies the legacy preflight contract
	// that initialized BPE tokens by rune boundary. It is retained only to
	// model historical configuration identity in regression tests.
	AIInputTokenEstimatorVersionV1 = "qwen-bpe-conservative-v1"
	// AIInputTokenEstimatorVersionV2 identifies the current immutable preflight
	// counting contract. Changing any input to its digest requires a new
	// candidate configuration.
	AIInputTokenEstimatorVersionV2 = "qwen-bpe-conservative-v2"

	qwenBPEVocabularySHA256  = "b2b1b8dfb5cc5f024bafc373121c6aba3f66f9a5a0269e243470a1de16a33186"
	qwenBPEVocabularyEntries = 151643
	qwenChatEnvelopeTokens   = 512
)

//go:embed assets/qwen.tiktoken.gz
var qwenBPEVocabularyGzip []byte

var (
	qwenBPERanksOnce sync.Once
	qwenBPERanks     map[string]int
	qwenBPERanksErr  error
)

// AIInputTokenEstimatorSHA256 identifies the non-secret, reproducible
// algorithm used to decide whether a request may leave the CVM. The digest is
// deliberately separate from the prompt/schema digests: those assets are
// already frozen independently in every AI configuration.
func AIInputTokenEstimatorSHA256() string {
	return aiInputTokenEstimatorSHA256(AIInputTokenEstimatorVersionV2)
}

func aiInputTokenEstimatorSHA256(version string) string {
	return sha256Hex(strings.Join([]string{
		version,
		"vocabulary_sha256=" + qwenBPEVocabularySHA256,
		"segmentation=ascii_letter_runs_and_single_non_ascii_or_non_letter_units-v1",
		"bpe=qwen_tiktoken_lowest_rank_merge-v1",
		fmt.Sprintf("chat_envelope_tokens=%d", qwenChatEnvelopeTokens),
	}, "\n"))
}

func conservativeQwenTokenCount(text string) (int, error) {
	ranks, err := qwenBPEVocabulary()
	if err != nil {
		return 0, err
	}

	tokens := 0
	for _, segment := range conservativeQwenSegments(text) {
		count, err := qwenBPECountSegment(segment, ranks)
		if err != nil {
			return 0, err
		}
		tokens += count
	}
	return tokens, nil
}

// conservativeQwenSegments deliberately adds boundaries compared with Qwen's
// normal pre-tokenizer. BPE merging can only reduce a segment's token count,
// so these additional boundaries give a reproducible upper bound while still
// allowing useful merges inside ASCII words and each Unicode code point.
func conservativeQwenSegments(text string) []string {
	segments := make([]string, 0, len(text)/4+1)
	for start := 0; start < len(text); {
		end := start
		if isASCIILetter(text[start]) {
			for end < len(text) && isASCIILetter(text[end]) {
				end++
			}
		} else {
			_, width := utf8.DecodeRuneInString(text[start:])
			end += width
		}
		segments = append(segments, text[start:end])
		start = end
	}
	return segments
}

func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z'
}

func qwenBPECountSegment(segment string, ranks map[string]int) (int, error) {
	tokens := make([]string, len(segment))
	for index := 0; index < len(segment); index++ {
		token := segment[index : index+1]
		if _, found := ranks[token]; !found {
			return 0, fmt.Errorf("Qwen BPE vocabulary does not contain an input byte")
		}
		tokens[index] = token
	}
	for len(tokens) > 1 {
		mergeToken, found := qwenBPELowestRankPair(tokens, ranks)
		if !found {
			break
		}
		tokens = qwenBPEMergeAll(tokens, mergeToken)
	}
	return len(tokens), nil
}

func qwenBPELowestRankPair(tokens []string, ranks map[string]int) (string, bool) {
	lowestRank := math.MaxInt
	lowestPair := ""
	for index := 0; index < len(tokens)-1; index++ {
		pair := tokens[index] + tokens[index+1]
		if rank, found := ranks[pair]; found && rank < lowestRank {
			lowestRank = rank
			lowestPair = pair
		}
	}
	return lowestPair, lowestPair != ""
}

func qwenBPEMergeAll(tokens []string, pair string) []string {
	merged := make([]string, 0, len(tokens))
	for index := 0; index < len(tokens); {
		if index+1 < len(tokens) && tokens[index]+tokens[index+1] == pair {
			merged = append(merged, pair)
			index += 2
			continue
		}
		merged = append(merged, tokens[index])
		index++
	}
	return merged
}

func qwenBPEVocabulary() (map[string]int, error) {
	qwenBPERanksOnce.Do(func() {
		qwenBPERanks, qwenBPERanksErr = loadQwenBPEVocabulary(qwenBPEVocabularyGzip)
	})
	return qwenBPERanks, qwenBPERanksErr
}

func loadQwenBPEVocabulary(compressed []byte) (map[string]int, error) {
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, fmt.Errorf("open Qwen BPE vocabulary: %w", err)
	}
	decompressed, err := ioReadAllAndClose(reader)
	if err != nil {
		return nil, fmt.Errorf("read Qwen BPE vocabulary: %w", err)
	}
	actualDigest := fmt.Sprintf("%x", sha256.Sum256(decompressed))
	if actualDigest != qwenBPEVocabularySHA256 {
		return nil, fmt.Errorf("Qwen BPE vocabulary digest mismatch")
	}

	ranks := make(map[string]int, qwenBPEVocabularyEntries)
	scanner := bufio.NewScanner(bytes.NewReader(decompressed))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) != 2 {
			return nil, fmt.Errorf("Qwen BPE vocabulary format is invalid")
		}
		decoded, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil || len(decoded) == 0 {
			return nil, fmt.Errorf("Qwen BPE vocabulary token is invalid")
		}
		var rank int
		if _, err := fmt.Sscanf(parts[1], "%d", &rank); err != nil || rank < 0 {
			return nil, fmt.Errorf("Qwen BPE vocabulary rank is invalid")
		}
		token := string(decoded)
		if _, duplicate := ranks[token]; duplicate {
			return nil, fmt.Errorf("Qwen BPE vocabulary contains a duplicate token")
		}
		ranks[token] = rank
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan Qwen BPE vocabulary: %w", err)
	}
	if len(ranks) != qwenBPEVocabularyEntries {
		return nil, fmt.Errorf("Qwen BPE vocabulary entry count is invalid")
	}
	return ranks, nil
}

func ioReadAllAndClose(reader *gzip.Reader) ([]byte, error) {
	decompressed, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return nil, err
	}
	return decompressed, closeErr
}
