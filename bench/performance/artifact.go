package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

const (
	performanceArtifactSchemaVersion = 1
	minimumArtifactRuns              = 3
)

type benchmarkArtifact struct {
	SchemaVersion int                 `json:"schema_version"`
	GeneratedAt   time.Time           `json:"generated_at"`
	Revision      string              `json:"revision"`
	Environment   string              `json:"environment"`
	Runs          [][]benchmarkReport `json:"runs"`
}

func newBenchmarkArtifact(runs [][]benchmarkReport, revision, environment string, generatedAt time.Time) (benchmarkArtifact, error) {
	artifact := benchmarkArtifact{
		SchemaVersion: performanceArtifactSchemaVersion,
		GeneratedAt:   generatedAt.UTC(),
		Revision:      revision,
		Environment:   environment,
		Runs:          runs,
	}
	if err := validateBenchmarkArtifact(artifact); err != nil {
		return benchmarkArtifact{}, err
	}
	return artifact, nil
}

func validateBenchmarkArtifact(artifact benchmarkArtifact) error {
	if artifact.SchemaVersion != performanceArtifactSchemaVersion {
		return fmt.Errorf("unsupported performance artifact schema %d", artifact.SchemaVersion)
	}
	if artifact.GeneratedAt.IsZero() {
		return errors.New("performance artifact generation time is required")
	}
	if artifact.Revision == "" {
		return errors.New("performance artifact revision is required")
	}
	if artifact.Environment == "" {
		return errors.New("performance artifact environment is required")
	}
	if len(artifact.Runs) < minimumArtifactRuns {
		return fmt.Errorf("performance artifact requires at least %d runs", minimumArtifactRuns)
	}

	want, err := artifactRunShapes(artifact.Runs[0])
	if err != nil {
		return fmt.Errorf("run 1: %w", err)
	}
	for index, run := range artifact.Runs[1:] {
		got, err := artifactRunShapes(run)
		if err != nil {
			return fmt.Errorf("run %d: %w", index+2, err)
		}
		if err := compareArtifactShapes(want, got); err != nil {
			return fmt.Errorf("run %d: %w", index+2, err)
		}
	}
	return nil
}

type artifactReportShape struct {
	System       systemMetadata
	Load         loadConfig
	PayloadBytes int
	MediaBytes   int
	HasTTFT      bool
}

func artifactRunShapes(reports []benchmarkReport) (map[string]artifactReportShape, error) {
	if len(reports) == 0 {
		return nil, errors.New("must contain at least one scenario")
	}
	shapes := make(map[string]artifactReportShape, len(reports))
	for _, report := range reports {
		if report.Scenario == "" {
			return nil, errors.New("scenario name is required")
		}
		if _, exists := shapes[report.Scenario]; exists {
			return nil, fmt.Errorf("duplicate scenario %q", report.Scenario)
		}
		shapes[report.Scenario] = artifactReportShape{
			System:       report.System,
			Load:         report.Load,
			PayloadBytes: report.PayloadBytes,
			MediaBytes:   report.MediaBytes,
			HasTTFT:      report.AddedTTFT.Samples > 0,
		}
	}
	return shapes, nil
}

func compareArtifactShapes(want, got map[string]artifactReportShape) error {
	if len(got) != len(want) {
		return fmt.Errorf("scenario count = %d, want %d", len(got), len(want))
	}
	for scenario, wantShape := range want {
		gotShape, exists := got[scenario]
		if !exists {
			return fmt.Errorf("missing scenario %q", scenario)
		}
		if gotShape != wantShape {
			return fmt.Errorf("scenario %q metadata differs between runs", scenario)
		}
	}
	return nil
}

func writeBenchmarkArtifact(path string, artifact benchmarkArtifact) error {
	if err := validateBenchmarkArtifact(artifact); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create performance artifact: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(artifact)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode performance artifact: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close performance artifact: %w", closeErr)
	}
	return nil
}

func readBenchmarkArtifact(path string) (benchmarkArtifact, error) {
	file, err := os.Open(path)
	if err != nil {
		return benchmarkArtifact{}, fmt.Errorf("open performance artifact: %w", err)
	}
	defer file.Close()

	var artifact benchmarkArtifact
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&artifact); err != nil {
		return benchmarkArtifact{}, fmt.Errorf("decode performance artifact: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return benchmarkArtifact{}, errors.New("performance artifact contains multiple JSON values")
		}
		return benchmarkArtifact{}, fmt.Errorf("decode performance artifact trailer: %w", err)
	}
	if err := validateBenchmarkArtifact(artifact); err != nil {
		return benchmarkArtifact{}, err
	}
	return artifact, nil
}
