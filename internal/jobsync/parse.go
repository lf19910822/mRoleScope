package jobsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"mrolescope/internal/collector"
)

// ParseObservations reads only files already validated against the manifest.
// It never accepts an arbitrary task directory as source evidence.
func ParseObservations(root string, manifest collector.Manifest) ([]Observation, error) {
	// The collector runner validates these before returning, but this parser is
	// also used as a persistence boundary. Repeating the checks here keeps an
	// accidental direct caller from turning an unverified directory into
	// publishable observations.
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("validate manifest: %w", err)
	}
	if err := collector.ValidateFiles(root, manifest); err != nil {
		return nil, fmt.Errorf("validate manifest files: %w", err)
	}
	listItems := make(map[string]map[string]any)
	detailFiles := make(map[string]string)
	for _, file := range manifest.Files {
		switch {
		case strings.HasPrefix(file.Path, "raw/list/"):
			items, err := readListItems(filepath.Join(root, file.Path))
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", file.Path, err)
			}
			for id, item := range items {
				if _, duplicate := listItems[id]; duplicate {
					return nil, fmt.Errorf("job %s appears in more than one list page", id)
				}
				listItems[id] = item
			}
		case strings.HasPrefix(file.Path, "raw/details/") && strings.HasSuffix(file.Path, ".json"):
			id := strings.TrimSuffix(filepath.Base(file.Path), ".json")
			if id == "" {
				return nil, fmt.Errorf("detail file has no job id: %s", file.Path)
			}
			detailFiles[id] = filepath.Join(root, file.Path)
		}
	}
	if len(listItems) != manifest.Counts.ListItems || len(detailFiles) != manifest.Counts.DetailSucceeded {
		return nil, fmt.Errorf("validated manifest does not contain all expected list/detail files")
	}

	ids := make([]string, 0, len(detailFiles))
	for id := range detailFiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	observations := make([]Observation, 0, len(ids))
	for _, id := range ids {
		listItem, exists := listItems[id]
		if !exists {
			return nil, fmt.Errorf("detail %s has no corresponding list item", id)
		}
		detail, err := readDataObject(detailFiles[id])
		if err != nil {
			return nil, fmt.Errorf("read detail %s: %w", id, err)
		}
		observation, err := NormalizeObservation(manifest.Source.Company, listItem, detail, manifest.FinishedAt)
		if err != nil {
			return nil, err
		}
		if observation.SourceJobID != id {
			return nil, fmt.Errorf("detail file id %s does not match response id %s", id, observation.SourceJobID)
		}
		observations = append(observations, observation)
	}
	if len(observations) != manifest.Counts.DetailSucceeded {
		return nil, fmt.Errorf("parsed observation count is incomplete")
	}
	return observations, nil
}

func readListItems(path string) (map[string]map[string]any, error) {
	data, err := readDataObject(path)
	if err != nil {
		return nil, err
	}
	rawItems, ok := data["list"].([]any)
	if !ok {
		return nil, fmt.Errorf("list data does not contain an array")
	}
	items := make(map[string]map[string]any, len(rawItems))
	for _, rawItem := range rawItems {
		item, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("list item is not an object")
		}
		id := normalizedString(item["id"])
		if id == "" {
			return nil, fmt.Errorf("list item has no id")
		}
		if _, duplicate := items[id]; duplicate {
			return nil, fmt.Errorf("list page contains duplicate job id %s", id)
		}
		items[id] = item
	}
	return items, nil
}

func readDataObject(path string) (map[string]any, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var response struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(contents, &response); err != nil {
		return nil, err
	}
	if !response.Success || response.Data == nil {
		return nil, fmt.Errorf("response is not a successful data object")
	}
	return response.Data, nil
}
