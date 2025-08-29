package movable

import (
	"testing"
)

// TestConfigBuilderV2 tests the configuration builder pattern
func TestConfigBuilderV2(t *testing.T) {
	// Test DirectFK configuration
	config, err := NewConfigBuilderV2().
		ForDirectFK("collections", "workspace_id").
		WithValidation(ValidationRulesV2{
			ValidateParentBoundaries: true,
			MaxItemsPerParent:       100,
		}).
		WithPerformance(PerformanceConfigV2{
			BatchSize:            50,
			UseCompiledQueries:   true,
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build DirectFK config: %v", err)
	}

	// Verify configuration was built correctly
	if config.ParentScope.Pattern != DirectFKPatternV2 {
		t.Errorf("Expected DirectFK pattern, got %v", config.ParentScope.Pattern)
	}

	if config.ParentScope.DirectFK.ParentColumn != "workspace_id" {
		t.Errorf("Expected ParentColumn workspace_id, got %s", config.ParentScope.DirectFK.ParentColumn)
	}

	if config.ValidationConfig.Rules.MaxItemsPerParent != 100 {
		t.Errorf("Expected MaxItemsPerParent 100, got %d", config.ValidationConfig.Rules.MaxItemsPerParent)
	}

	if config.PerformanceConfig.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50, got %d", config.PerformanceConfig.BatchSize)
	}
}

// TestJoinTableConfig tests join table configuration
func TestJoinTableConfig(t *testing.T) {
	config, err := NewConfigBuilderV2().
		ForJoinTable("workspaces", "workspace_users", "workspace_id", "user_id").
		WithValidation(ValidationRulesV2{
			ValidatePermissions: true,
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build JoinTable config: %v", err)
	}

	if config.ParentScope.Pattern != JoinTablePatternV2 {
		t.Errorf("Expected JoinTable pattern, got %v", config.ParentScope.Pattern)
	}

	if config.ParentScope.JoinTable.JoinTableName != "workspace_users" {
		t.Errorf("Expected JoinTableName workspace_users, got %s", config.ParentScope.JoinTable.JoinTableName)
	}

	if config.ParentScope.JoinTable.EntityColumn != "workspace_id" {
		t.Errorf("Expected EntityColumn workspace_id, got %s", config.ParentScope.JoinTable.EntityColumn)
	}

	if config.ParentScope.JoinTable.ParentColumn != "user_id" {
		t.Errorf("Expected ParentColumn user_id, got %s", config.ParentScope.JoinTable.ParentColumn)
	}
}

// TestUserLookupConfig tests user lookup configuration
func TestUserLookupConfig(t *testing.T) {
	config, err := NewConfigBuilderV2().
		ForUserLookup("variables", "user_id").
		WithValidation(ValidationRulesV2{
			ValidateUserAccess: true,
			MaxItemsPerParent: 500,
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build UserLookup config: %v", err)
	}

	if config.ParentScope.Pattern != UserLookupPatternV2 {
		t.Errorf("Expected UserLookup pattern, got %v", config.ParentScope.Pattern)
	}

	if config.ParentScope.UserLookup.UserIDColumn != "user_id" {
		t.Errorf("Expected UserIDColumn user_id, got %s", config.ParentScope.UserLookup.UserIDColumn)
	}

	if !config.ValidationConfig.Rules.ValidateUserAccess {
		t.Error("Expected ValidateUserAccess to be true")
	}
}

// TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	// Test missing required fields
	_, err := NewConfigBuilderV2().Build()
	if err == nil {
		t.Error("Expected validation error for incomplete config")
	}

	// Test DirectFK without ParentColumn
	builder := NewConfigBuilderV2()
	builder.config.ParentScope.Pattern = DirectFKPatternV2
	// Don't set ParentColumn
	_, err = builder.Build()
	if err == nil {
		t.Error("Expected validation error for DirectFK without ParentColumn")
	}
}

// TestDefaultConfigurations tests default configuration values
func TestDefaultConfigurations(t *testing.T) {
	transactionConfig := DefaultTransactionConfigV2()
	if transactionConfig.IsolationLevel != "READ_COMMITTED" {
		t.Errorf("Expected READ_COMMITTED isolation, got %s", transactionConfig.IsolationLevel)
	}

	performanceConfig := DefaultPerformanceConfigV2()
	if performanceConfig.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50, got %d", performanceConfig.BatchSize)
	}

	validationRules := DefaultValidationRulesV2()
	if !validationRules.ValidateParentBoundaries {
		t.Error("Expected ValidateParentBoundaries to be true by default")
	}
}