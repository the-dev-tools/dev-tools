package ioworkspace

import (
	"context"
	"fmt"

	"github.com/the-dev-tools/dev-tools/packages/server/pkg/idwrap"
	"github.com/the-dev-tools/dev-tools/packages/server/pkg/service/senv"
)

// importEnvironments imports environments from the bundle.
func (s *IOWorkspaceService) importEnvironments(ctx context.Context, envService senv.EnvironmentService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, env := range bundle.Environments {
		oldID := env.ID

		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			env.ID = idwrap.NewNow()
		}

		// Update workspace ID
		env.WorkspaceID = opts.WorkspaceID

		// Create environment
		if err := envService.CreateEnvironment(ctx, &env); err != nil {
			return fmt.Errorf("failed to create environment %s: %w", env.Name, err)
		}

		// Track ID mapping
		result.EnvironmentIDMap[oldID] = env.ID
		result.EnvironmentsCreated++
	}
	return nil
}

// importEnvironmentVars imports environment variables from the bundle.
func (s *IOWorkspaceService) importEnvironmentVars(ctx context.Context, varService senv.VariableService, bundle *WorkspaceBundle, opts ImportOptions, result *ImportResult) error {
	for _, envVar := range bundle.EnvironmentVars {
		// Generate new ID if not preserving
		if !opts.PreserveIDs {
			envVar.ID = idwrap.NewNow()
		}

		// Remap environment ID
		if newEnvID, ok := result.EnvironmentIDMap[envVar.EnvID]; ok {
			envVar.EnvID = newEnvID
		}

		// Create environment variable
		if err := varService.Create(ctx, envVar); err != nil {
			return fmt.Errorf("failed to create environment variable %s: %w", envVar.VarKey, err)
		}

		result.EnvironmentVarsCreated++
	}
	return nil
}
