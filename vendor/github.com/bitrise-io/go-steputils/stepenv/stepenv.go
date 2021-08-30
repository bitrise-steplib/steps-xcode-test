package stepenv

import (
	"github.com/bitrise-io/go-steputils/tools"
	"github.com/bitrise-io/go-utils/env"
)

// NewRepository ...
func NewRepository(osRepository env.Repository) env.Repository {
	return defaultRepository{osRepository: osRepository}
}

type defaultRepository struct {
	osRepository env.Repository
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
	return tools.ExportEnvironmentWithEnvman(key, value)
}

// Unset ...
func (r defaultRepository) Unset(key string) error {
	if err := r.osRepository.Unset(key); err != nil {
		return err
	}
	return tools.ExportEnvironmentWithEnvman(key, "")
}

// List ...
func (r defaultRepository) List() []string {
	return r.osRepository.List()
}
