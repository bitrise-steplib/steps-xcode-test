package stepenv

import (
	"github.com/bitrise-io/go-steputils/v2/export"
	"github.com/bitrise-io/go-utils/v2/command"
	"github.com/bitrise-io/go-utils/v2/env"
)

// NewRepository ...
func NewRepository(osRepository env.Repository) env.Repository {
	return defaultRepository{
		osRepository: osRepository,
		exporter:     export.NewExporter(command.NewFactory(osRepository)),
	}
}

type defaultRepository struct {
	osRepository env.Repository
	exporter     export.Exporter
}

// Get ...
func (r defaultRepository) Get(key string) string {
	return r.osRepository.Get(key)
}

// Set ...
func (r defaultRepository) Set(key, value string) error {
	if err := r.osRepository.Set(key, value); err != nil {
		return err
	}
	return r.exporter.ExportOutput(key, value)
}

// Unset ...
func (r defaultRepository) Unset(key string) error {
	if err := r.osRepository.Unset(key); err != nil {
		return err
	}
	return r.exporter.ExportOutput(key, "")
}

// List ...
func (r defaultRepository) List() []string {
	return r.osRepository.List()
}
