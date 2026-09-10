package buffer

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/GreenSupplyChain-DIH/cow-collector/internal/models"
)

type JSONLBuffer struct {
	path string
}

func New(path string) *JSONLBuffer {
	return &JSONLBuffer{path: path}
}

func (b *JSONLBuffer) Store(_ context.Context, records []models.TelemetryRecord) error {
	if len(records) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(b.path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(b.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, record := range records {
		if err := encoder.Encode(record); err != nil {
			return err
		}
	}
	return nil
}

func (b *JSONLBuffer) Load(_ context.Context) ([]models.TelemetryRecord, error) {
	file, err := os.Open(b.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(bufio.NewReader(file))
	records := make([]models.TelemetryRecord, 0)
	for {
		var record models.TelemetryRecord
		if err := decoder.Decode(&record); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

func (b *JSONLBuffer) Reset(_ context.Context) error {
	if err := os.Remove(b.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
