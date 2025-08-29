package movable

import (
	"context"
	"fmt"
	"time"

	"the-dev-tools/server/pkg/idwrap"
)

// Enhanced configuration structs that complement the existing base types
// These are designed to work alongside the existing ParentScopeConfig and EntityData

// ========== Enhanced Configuration Types ==========

// MoveConfigV2 contains comprehensive move operation configuration
type MoveConfigV2 struct {
	// Function-based operations (replacing template methods)
	EntityOperations   EntityOperationsV2
	QueryOperations    QueryOperationsV2
	ValidationOps      ValidationOperationsV2
	
	// Database configuration
	TableConfig        TableConfigV2
	ParentScope        ParentScopeConfigV2
	TransactionConfig  TransactionConfigV2
	
	// Performance and validation tuning
	PerformanceConfig  PerformanceConfigV2
	ValidationConfig   ValidationConfigV2
}

// EntityOperationsV2 replaces LinkedListOperations interface
type EntityOperationsV2 struct {
	GetEntity       GetEntityFuncV2
	UpdateOrder     UpdateOrderFuncV2
	ExtractData     ExtractDataFuncV2
	ValidateEntity  ValidateEntityFuncV2
}

// QueryOperationsV2 replaces query-related template methods
type QueryOperationsV2 struct {
	BuildOrderedQuery BuildOrderedQueryFuncV2
	BuildScopeClause  BuildScopeClauseFuncV2
	BuildCountQuery   BuildCountQueryFuncV2
	CustomQueryHints  map[string]string
}

// ValidationOperationsV2 replaces validation logic
type ValidationOperationsV2 struct {
	ValidateParent     ValidateParentFuncV2
	ValidatePosition   ValidatePositionFuncV2
	ValidatePermission ValidatePermissionFuncV2
	ValidateIntegrity  ValidateIntegrityFuncV2
}

// ========== Function Type Definitions V2 ==========

// Entity operation functions
type GetEntityFuncV2 func(ctx context.Context, queries interface{}, id idwrap.IDWrap) (interface{}, error)
type UpdateOrderFuncV2 func(ctx context.Context, queries interface{}, id idwrap.IDWrap, prev, next *idwrap.IDWrap) error
type ExtractDataFuncV2 func(entity interface{}) EntityDataV2
type ValidateEntityFuncV2 func(ctx context.Context, entity interface{}) error

// Query building functions
type BuildOrderedQueryFuncV2 func(config QueryConfigV2) (query string, args []interface{})
type BuildScopeClauseFuncV2 func(parentID idwrap.IDWrap, config ParentScopeConfigV2) (clause string, args []interface{})
type BuildCountQueryFuncV2 func(parentID idwrap.IDWrap, config QueryConfigV2) (query string, args []interface{})

// Validation functions
type ValidateParentFuncV2 func(ctx context.Context, itemID, parentID idwrap.IDWrap) error
type ValidatePositionFuncV2 func(ctx context.Context, parentID idwrap.IDWrap, position int) error
type ValidatePermissionFuncV2 func(ctx context.Context, userID, itemID idwrap.IDWrap, operation string) error
type ValidateIntegrityFuncV2 func(ctx context.Context, items []LinkedListPointersV2) error

// Permission and access control functions
type PermissionCheckFuncV2 func(ctx context.Context, userID, entityID idwrap.IDWrap, role string) (bool, error)
type UserAccessCheckFuncV2 func(ctx context.Context, userID, entityID idwrap.IDWrap) (bool, error)

// LinkedListPointersV2 represents the pointer structure for validation
type LinkedListPointersV2 struct {
	ID   idwrap.IDWrap
	Prev *idwrap.IDWrap
	Next *idwrap.IDWrap
}

// EntityDataV2 represents extracted data from any entity (compatible with existing EntityData)
type EntityDataV2 struct {
	ID       idwrap.IDWrap
	Prev     *idwrap.IDWrap
	Next     *idwrap.IDWrap
	ParentID idwrap.IDWrap
	Position int
}

// ========== Configuration Structs V2 ==========

// ValidationConfigV2 contains validation settings
type ValidationConfigV2 struct {
	Rules ValidationRulesV2
}

// TransactionConfigV2 defines transaction behavior
type TransactionConfigV2 struct {
	IsolationLevel      string
	TimeoutDuration     time.Duration
	RetryAttempts       int
	RetryBackoff        time.Duration
	EnableDeadlockRetry bool
}

// PerformanceConfigV2 defines performance optimization settings
type PerformanceConfigV2 struct {
	BatchSize            int
	UseCompiledQueries   bool
	EnableQueryCaching   bool
	CacheExpiration      time.Duration
	MaxConcurrentOps     int
	EnableBulkOperations bool
	QueryTimeout         time.Duration
}

// ValidationRulesV2 defines validation configuration
type ValidationRulesV2 struct {
	ValidateParentBoundaries bool
	ValidatePermissions      bool
	ValidateUserAccess       bool
	RequireActiveStatus      bool
	MaxItemsPerParent        int
	MinItemsPerParent        int
	AllowEmptyParents        bool
}

// QueryConfigV2 contains query building parameters
type QueryConfigV2 struct {
	TableName      string
	IDColumn       string
	PrevColumn     string
	NextColumn     string
	PositionColumn string
	ParentID       idwrap.IDWrap
	ParentScope    ParentScopeConfigV2
	Limit          int
	Offset         int
	OrderBy        string
	CustomFilters  map[string]interface{}
}

// TableConfigV2 defines database table schema mapping
type TableConfigV2 struct {
	TableName      string // Primary table name
	IDColumn       string // Primary key column
	PrevColumn     string // Previous item pointer column
	NextColumn     string // Next item pointer column
	PositionColumn string // Position/order column
}

// ========== Parent Scope Configuration V2 ==========

// ParentScopePattern defines the type of parent-child relationship pattern
type ParentScopePatternV2 int

const (
	DirectFKPatternV2 ParentScopePatternV2 = iota
	JoinTablePatternV2
	UserLookupPatternV2
)

// CascadeAction defines what happens when parent is deleted
type CascadeActionV2 int

const (
	CascadeRestrictV2 CascadeActionV2 = iota
	CascadeDeleteV2
	CascadeSetNullV2
)

// JoinType defines the type of JOIN operation
type JoinTypeV2 int

const (
	InnerJoinV2 JoinTypeV2 = iota
	LeftJoinV2
	RightJoinV2
	FullJoinV2
)

// TenantStrategy defines multi-tenant isolation strategy
type TenantStrategyV2 int

const (
	TenantByColumnV2 TenantStrategyV2 = iota
	TenantBySchemaV2
	TenantByDatabaseV2
)

// ParentScopeConfigV2 replaces strategy enum with data-driven approach
type ParentScopeConfigV2 struct {
	// Scope resolution pattern
	Pattern ParentScopePatternV2
	
	// Pattern-specific configurations
	DirectFK   DirectFKConfigV2
	JoinTable  JoinTableConfigV2
	UserLookup UserLookupConfigV2
	
	// Common scope settings
	EnableCaching     bool
	CacheExpiration   time.Duration
	ParentValidation  ParentValidationConfigV2
}

// DirectFKConfigV2 for direct foreign key relationships
type DirectFKConfigV2 struct {
	// Column configuration
	ParentColumn     string            // e.g., "env_id", "collection_id"
	ParentTable      string            // e.g., "environments", "collections"
	
	// Relationship configuration
	OnParentDelete   CascadeActionV2   // CASCADE, RESTRICT, SET_NULL
	IndexHint        string            // Optional index hint for queries
	
	// Validation
	ValidateParentExists bool          // Check parent exists before operations
	AllowNullParent     bool           // Allow NULL parent (root items)
	
	// Custom query modifiers
	CustomWhereClause   string         // Additional WHERE conditions
	CustomJoinClause    string         // Additional JOIN conditions
}

// JoinTableConfigV2 for many-to-many relationships
type JoinTableConfigV2 struct {
	// Table structure
	JoinTableName   string            // e.g., "workspace_users"
	EntityColumn    string            // e.g., "workspace_id"
	ParentColumn    string            // e.g., "user_id"
	
	// Join configuration
	JoinType        JoinTypeV2        // INNER, LEFT, RIGHT, FULL
	ActiveColumn    string            // Optional: filter by active status
	RoleColumn      string            // Optional: role-based filtering
	
	// Performance optimization
	UseSubquery     bool              // Use subquery vs JOIN for large tables
	IndexHint       string            // Index hint for join operations
	
	// Access control
	RequiredRole    string                // Required role for access
	PermissionCheck PermissionCheckFuncV2
}

// UserLookupConfigV2 for user-scoped entities
type UserLookupConfigV2 struct {
	// User identification
	UserIDColumn    string                // e.g., "user_id"
	TenantColumn    string                // e.g., "tenant_id" (optional)
	
	// User resolution
	UserResolution  UserResolutionStrategyV2
	
	// Multi-tenancy support
	TenantIsolation bool
	TenantStrategy  TenantStrategyV2
	
	// Security
	ValidateUserAccess bool
	UserAccessCheck    UserAccessCheckFuncV2
}

// UserResolutionStrategyV2 defines how to resolve user context
type UserResolutionStrategyV2 struct {
	FromContext bool
	ContextKey  string
	FromHeader  bool
	HeaderName  string
	FromCookie  bool
	CookieName  string
}

// ParentValidationConfigV2 defines parent entity validation
type ParentValidationConfigV2 struct {
	CheckExists     bool
	CheckActive     bool
	CheckPermission bool
	CustomValidator ValidateParentFuncV2
}

// ========== Configuration Builder V2 ==========

// ConfigBuilderV2 provides fluent interface for building configurations
type ConfigBuilderV2 struct {
	config MoveConfigV2
	err    error
}

// NewConfigBuilderV2 creates a new configuration builder
func NewConfigBuilderV2() *ConfigBuilderV2 {
	return &ConfigBuilderV2{
		config: MoveConfigV2{
			TransactionConfig: DefaultTransactionConfigV2(),
			PerformanceConfig: DefaultPerformanceConfigV2(),
		},
	}
}

// ForDirectFK configures direct foreign key pattern
func (b *ConfigBuilderV2) ForDirectFK(tableName, parentColumn string) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.TableConfig = TableConfigV2{
		TableName:      tableName,
		IDColumn:       "id",
		PrevColumn:     "prev",
		NextColumn:     "next",
		PositionColumn: "position",
	}
	
	b.config.ParentScope = ParentScopeConfigV2{
		Pattern: DirectFKPatternV2,
		DirectFK: DirectFKConfigV2{
			ParentColumn:         parentColumn,
			ValidateParentExists: true,
			OnParentDelete:       CascadeRestrictV2,
		},
	}
	
	// Set default functions for DirectFK pattern
	b.config.EntityOperations = defaultDirectFKEntityOpsV2(b.config.TableConfig)
	b.config.QueryOperations = defaultDirectFKQueryOpsV2(b.config.TableConfig)
	
	return b
}

// ForJoinTable configures join table pattern
func (b *ConfigBuilderV2) ForJoinTable(entityTable, joinTable, entityCol, parentCol string) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.TableConfig = TableConfigV2{
		TableName:      entityTable,
		IDColumn:       "id",
		PrevColumn:     "prev",
		NextColumn:     "next",
		PositionColumn: "position",
	}
	
	b.config.ParentScope = ParentScopeConfigV2{
		Pattern: JoinTablePatternV2,
		JoinTable: JoinTableConfigV2{
			JoinTableName: joinTable,
			EntityColumn:  entityCol,
			ParentColumn:  parentCol,
			JoinType:      InnerJoinV2,
		},
	}
	
	// Set default functions for JoinTable pattern
	b.config.EntityOperations = defaultJoinTableEntityOpsV2(b.config.TableConfig)
	b.config.QueryOperations = defaultJoinTableQueryOpsV2(b.config.TableConfig, b.config.ParentScope.JoinTable)
	
	return b
}

// ForUserLookup configures user-scoped pattern
func (b *ConfigBuilderV2) ForUserLookup(tableName, userColumn string) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.TableConfig = TableConfigV2{
		TableName:      tableName,
		IDColumn:       "id",
		PrevColumn:     "prev",
		NextColumn:     "next",
		PositionColumn: "position",
	}
	
	b.config.ParentScope = ParentScopeConfigV2{
		Pattern: UserLookupPatternV2,
		UserLookup: UserLookupConfigV2{
			UserIDColumn:       userColumn,
			ValidateUserAccess: true,
			UserResolution: UserResolutionStrategyV2{
				FromContext: true,
				ContextKey:  "user_id",
			},
		},
	}
	
	// Set default functions for UserLookup pattern
	b.config.EntityOperations = defaultUserLookupEntityOpsV2(b.config.TableConfig)
	b.config.QueryOperations = defaultUserLookupQueryOpsV2(b.config.TableConfig, b.config.ParentScope.UserLookup)
	
	return b
}

// WithValidation adds validation configuration
func (b *ConfigBuilderV2) WithValidation(rules ValidationRulesV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.ValidationConfig = ValidationConfigV2{
		Rules: rules,
	}
	
	return b
}

// WithPerformance adds performance configuration
func (b *ConfigBuilderV2) WithPerformance(config PerformanceConfigV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.PerformanceConfig = config
	return b
}

// WithCustomEntityOps overrides entity operations
func (b *ConfigBuilderV2) WithCustomEntityOps(ops EntityOperationsV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.EntityOperations = ops
	return b
}

// WithCustomQueryOps overrides query operations
func (b *ConfigBuilderV2) WithCustomQueryOps(ops QueryOperationsV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.QueryOperations = ops
	return b
}

// WithCustomValidationOps overrides validation operations
func (b *ConfigBuilderV2) WithCustomValidationOps(ops ValidationOperationsV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.ValidationOps = ops
	return b
}

// WithTableConfig overrides table configuration
func (b *ConfigBuilderV2) WithTableConfig(config TableConfigV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.TableConfig = config
	return b
}

// WithTransactionConfig overrides transaction configuration
func (b *ConfigBuilderV2) WithTransactionConfig(config TransactionConfigV2) *ConfigBuilderV2 {
	if b.err != nil {
		return b
	}
	
	b.config.TransactionConfig = config
	return b
}

// Build finalizes the configuration
func (b *ConfigBuilderV2) Build() (MoveConfigV2, error) {
	if b.err != nil {
		return MoveConfigV2{}, b.err
	}
	
	// Validate configuration completeness
	if err := b.validateConfig(); err != nil {
		return MoveConfigV2{}, err
	}
	
	return b.config, nil
}

// validateConfig ensures configuration is complete and consistent
func (b *ConfigBuilderV2) validateConfig() error {
	if b.config.EntityOperations.GetEntity == nil {
		return fmt.Errorf("GetEntity function is required")
	}
	
	if b.config.EntityOperations.UpdateOrder == nil {
		return fmt.Errorf("UpdateOrder function is required")
	}
	
	if b.config.QueryOperations.BuildOrderedQuery == nil {
		return fmt.Errorf("BuildOrderedQuery function is required")
	}
	
	// Pattern-specific validation
	switch b.config.ParentScope.Pattern {
	case DirectFKPatternV2:
		if b.config.ParentScope.DirectFK.ParentColumn == "" {
			return fmt.Errorf("DirectFK ParentColumn is required")
		}
	case JoinTablePatternV2:
		if b.config.ParentScope.JoinTable.JoinTableName == "" {
			return fmt.Errorf("JoinTable JoinTableName is required")
		}
	case UserLookupPatternV2:
		if b.config.ParentScope.UserLookup.UserIDColumn == "" {
			return fmt.Errorf("UserLookup UserIDColumn is required")
		}
	}
	
	return nil
}

// ========== Default Configuration Functions ==========

// DefaultTransactionConfigV2 returns sensible defaults for transaction configuration
func DefaultTransactionConfigV2() TransactionConfigV2 {
	return TransactionConfigV2{
		IsolationLevel:      "READ_COMMITTED",
		TimeoutDuration:     30 * time.Second,
		RetryAttempts:       3,
		RetryBackoff:        100 * time.Millisecond,
		EnableDeadlockRetry: true,
	}
}

// DefaultPerformanceConfigV2 returns sensible defaults for performance configuration
func DefaultPerformanceConfigV2() PerformanceConfigV2 {
	return PerformanceConfigV2{
		BatchSize:            50,
		UseCompiledQueries:   true,
		EnableQueryCaching:   false,
		CacheExpiration:      5 * time.Minute,
		MaxConcurrentOps:     10,
		EnableBulkOperations: false,
		QueryTimeout:         10 * time.Second,
	}
}

// DefaultValidationRulesV2 returns sensible defaults for validation rules
func DefaultValidationRulesV2() ValidationRulesV2 {
	return ValidationRulesV2{
		ValidateParentBoundaries: true,
		ValidatePermissions:      false,
		ValidateUserAccess:       false,
		RequireActiveStatus:      false,
		MaxItemsPerParent:        1000,
		MinItemsPerParent:        0,
		AllowEmptyParents:        true,
	}
}

// ========== Default Implementation Functions ==========

// Helper function to convert nullable IDWrap pointer to bytes pointer
func nullableIDWrapV2(id *idwrap.IDWrap) *[]byte {
	if id == nil {
		return nil
	}
	bytes := id.Bytes()
	return &bytes
}

// Helper function to convert bytes pointer to nullable IDWrap
func nullableIDWrapFromBytesV2(bytes *[]byte) *idwrap.IDWrap {
	if bytes == nil {
		return nil
	}
	id, err := idwrap.NewFromBytes(*bytes)
	if err != nil {
		return nil
	}
	return &id
}

// defaultDirectFKEntityOpsV2 creates default entity operations for DirectFK pattern
func defaultDirectFKEntityOpsV2(tableConfig TableConfigV2) EntityOperationsV2 {
	return EntityOperationsV2{
		GetEntity: func(ctx context.Context, queries interface{}, id idwrap.IDWrap) (interface{}, error) {
			// This would be implemented based on actual SQLC generated methods
			// For now, return a placeholder error indicating implementation needed
			return nil, fmt.Errorf("GetEntity implementation needed for table: %s", tableConfig.TableName)
		},
		
		UpdateOrder: func(ctx context.Context, queries interface{}, id idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
			// This would be implemented based on actual SQLC generated methods
			// For now, return a placeholder error indicating implementation needed
			return fmt.Errorf("UpdateOrder implementation needed for table: %s", tableConfig.TableName)
		},
		
		ExtractData: func(entity interface{}) EntityDataV2 {
			// This would need type assertions based on actual generated types
			// For now, return empty data
			return EntityDataV2{}
		},
		
		ValidateEntity: func(ctx context.Context, entity interface{}) error {
			// Default validation can be no-op
			return nil
		},
	}
}

// defaultDirectFKQueryOpsV2 creates default query operations for DirectFK pattern
func defaultDirectFKQueryOpsV2(tableConfig TableConfigV2) QueryOperationsV2 {
	return QueryOperationsV2{
		BuildOrderedQuery: func(config QueryConfigV2) (string, []interface{}) {
			parentCol := config.ParentScope.DirectFK.ParentColumn
			
			query := fmt.Sprintf(`
				WITH RECURSIVE ordered_items AS (
					-- Base case: Find head item (prev IS NULL)
					SELECT 
						%s, %s, %s, %s, %s,
						0 as traversal_position
					FROM %s
					WHERE %s IS NULL AND %s = $1
					
					UNION ALL
					
					-- Recursive case: Follow next pointers
					SELECT 
						t.%s, t.%s, t.%s, t.%s, t.%s,
						oi.traversal_position + 1
					FROM %s t
					INNER JOIN ordered_items oi ON t.%s = oi.%s
					WHERE t.%s = $2
				)
				SELECT * FROM ordered_items
				ORDER BY traversal_position`,
				config.IDColumn, config.PrevColumn, config.NextColumn, 
				config.PositionColumn, parentCol,
				config.TableName,
				config.PrevColumn, parentCol,
				config.IDColumn, config.PrevColumn, config.NextColumn,
				config.PositionColumn, parentCol,
				config.TableName,
				config.PrevColumn, config.IDColumn,
				parentCol,
			)
			
			return query, []interface{}{config.ParentID.Bytes(), config.ParentID.Bytes()}
		},
		
		BuildScopeClause: func(parentID idwrap.IDWrap, config ParentScopeConfigV2) (string, []interface{}) {
			parentCol := config.DirectFK.ParentColumn
			clause := fmt.Sprintf("%s = ?", parentCol)
			args := []interface{}{parentID.Bytes()}
			
			// Add custom WHERE clause if specified
			if config.DirectFK.CustomWhereClause != "" {
				clause = fmt.Sprintf("(%s) AND (%s)", clause, config.DirectFK.CustomWhereClause)
			}
			
			return clause, args
		},
		
		BuildCountQuery: func(parentID idwrap.IDWrap, config QueryConfigV2) (string, []interface{}) {
			parentCol := config.ParentScope.DirectFK.ParentColumn
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", 
				config.TableName, parentCol)
			return query, []interface{}{parentID.Bytes()}
		},
		
		CustomQueryHints: make(map[string]string),
	}
}

// defaultJoinTableEntityOpsV2 creates default entity operations for JoinTable pattern
func defaultJoinTableEntityOpsV2(tableConfig TableConfigV2) EntityOperationsV2 {
	return EntityOperationsV2{
		GetEntity: func(ctx context.Context, queries interface{}, id idwrap.IDWrap) (interface{}, error) {
			return nil, fmt.Errorf("GetEntity implementation needed for join table pattern: %s", tableConfig.TableName)
		},
		
		UpdateOrder: func(ctx context.Context, queries interface{}, id idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
			return fmt.Errorf("UpdateOrder implementation needed for join table pattern: %s", tableConfig.TableName)
		},
		
		ExtractData: func(entity interface{}) EntityDataV2 {
			return EntityDataV2{}
		},
		
		ValidateEntity: func(ctx context.Context, entity interface{}) error {
			return nil
		},
	}
}

// defaultJoinTableQueryOpsV2 creates default query operations for JoinTable pattern
func defaultJoinTableQueryOpsV2(tableConfig TableConfigV2, joinConfig JoinTableConfigV2) QueryOperationsV2 {
	return QueryOperationsV2{
		BuildOrderedQuery: func(config QueryConfigV2) (string, []interface{}) {
			joinType := "INNER JOIN"
			switch joinConfig.JoinType {
			case LeftJoinV2:
				joinType = "LEFT JOIN"
			case RightJoinV2:
				joinType = "RIGHT JOIN"
			case FullJoinV2:
				joinType = "FULL JOIN"
			}
			
			query := fmt.Sprintf(`
				WITH RECURSIVE ordered_items AS (
					-- Base case: Find head item (prev IS NULL)
					SELECT 
						t.%s, t.%s, t.%s, t.%s,
						0 as traversal_position
					FROM %s t
					%s %s jt ON t.%s = jt.%s
					WHERE t.%s IS NULL AND jt.%s = $1
					
					UNION ALL
					
					-- Recursive case: Follow next pointers
					SELECT 
						t.%s, t.%s, t.%s, t.%s,
						oi.traversal_position + 1
					FROM %s t
					%s %s jt ON t.%s = jt.%s
					INNER JOIN ordered_items oi ON t.%s = oi.%s
					WHERE jt.%s = $2
				)
				SELECT * FROM ordered_items
				ORDER BY traversal_position`,
				config.IDColumn, config.PrevColumn, config.NextColumn, config.PositionColumn,
				config.TableName,
				joinType, joinConfig.JoinTableName, config.IDColumn, joinConfig.EntityColumn,
				config.PrevColumn, joinConfig.ParentColumn,
				config.IDColumn, config.PrevColumn, config.NextColumn, config.PositionColumn,
				config.TableName,
				joinType, joinConfig.JoinTableName, config.IDColumn, joinConfig.EntityColumn,
				config.PrevColumn, config.IDColumn,
				joinConfig.ParentColumn,
			)
			
			return query, []interface{}{config.ParentID.Bytes(), config.ParentID.Bytes()}
		},
		
		BuildScopeClause: func(parentID idwrap.IDWrap, config ParentScopeConfigV2) (string, []interface{}) {
			joinConfig := config.JoinTable
			clause := fmt.Sprintf(`id IN (
				SELECT %s 
				FROM %s 
				WHERE %s = ?`,
				joinConfig.EntityColumn,
				joinConfig.JoinTableName,
				joinConfig.ParentColumn,
			)
			
			args := []interface{}{parentID.Bytes()}
			
			// Add active column filter if specified
			if joinConfig.ActiveColumn != "" {
				clause += fmt.Sprintf(" AND %s = true", joinConfig.ActiveColumn)
			}
			
			// Add role filter if specified
			if joinConfig.RoleColumn != "" && joinConfig.RequiredRole != "" {
				clause += fmt.Sprintf(" AND %s = ?", joinConfig.RoleColumn)
				args = append(args, joinConfig.RequiredRole)
			}
			
			clause += ")"
			
			return clause, args
		},
		
		BuildCountQuery: func(parentID idwrap.IDWrap, config QueryConfigV2) (string, []interface{}) {
			joinConfig := config.ParentScope.JoinTable
			query := fmt.Sprintf(`
				SELECT COUNT(*) 
				FROM %s t
				INNER JOIN %s jt ON t.%s = jt.%s
				WHERE jt.%s = ?`,
				config.TableName,
				joinConfig.JoinTableName, config.IDColumn, joinConfig.EntityColumn,
				joinConfig.ParentColumn,
			)
			return query, []interface{}{parentID.Bytes()}
		},
		
		CustomQueryHints: make(map[string]string),
	}
}

// defaultUserLookupEntityOpsV2 creates default entity operations for UserLookup pattern
func defaultUserLookupEntityOpsV2(tableConfig TableConfigV2) EntityOperationsV2 {
	return EntityOperationsV2{
		GetEntity: func(ctx context.Context, queries interface{}, id idwrap.IDWrap) (interface{}, error) {
			return nil, fmt.Errorf("GetEntity implementation needed for user lookup pattern: %s", tableConfig.TableName)
		},
		
		UpdateOrder: func(ctx context.Context, queries interface{}, id idwrap.IDWrap, prev, next *idwrap.IDWrap) error {
			return fmt.Errorf("UpdateOrder implementation needed for user lookup pattern: %s", tableConfig.TableName)
		},
		
		ExtractData: func(entity interface{}) EntityDataV2 {
			return EntityDataV2{}
		},
		
		ValidateEntity: func(ctx context.Context, entity interface{}) error {
			return nil
		},
	}
}

// defaultUserLookupQueryOpsV2 creates default query operations for UserLookup pattern
func defaultUserLookupQueryOpsV2(tableConfig TableConfigV2, userConfig UserLookupConfigV2) QueryOperationsV2 {
	return QueryOperationsV2{
		BuildOrderedQuery: func(config QueryConfigV2) (string, []interface{}) {
			userCol := config.ParentScope.UserLookup.UserIDColumn
			
			query := fmt.Sprintf(`
				WITH RECURSIVE ordered_items AS (
					-- Base case: Find head item (prev IS NULL)
					SELECT 
						%s, %s, %s, %s, %s,
						0 as traversal_position
					FROM %s
					WHERE %s IS NULL AND %s = $1
					
					UNION ALL
					
					-- Recursive case: Follow next pointers
					SELECT 
						t.%s, t.%s, t.%s, t.%s, t.%s,
						oi.traversal_position + 1
					FROM %s t
					INNER JOIN ordered_items oi ON t.%s = oi.%s
					WHERE t.%s = $2
				)
				SELECT * FROM ordered_items
				ORDER BY traversal_position`,
				config.IDColumn, config.PrevColumn, config.NextColumn, 
				config.PositionColumn, userCol,
				config.TableName,
				config.PrevColumn, userCol,
				config.IDColumn, config.PrevColumn, config.NextColumn,
				config.PositionColumn, userCol,
				config.TableName,
				config.PrevColumn, config.IDColumn,
				userCol,
			)
			
			return query, []interface{}{config.ParentID.Bytes(), config.ParentID.Bytes()}
		},
		
		BuildScopeClause: func(parentID idwrap.IDWrap, config ParentScopeConfigV2) (string, []interface{}) {
			userCol := config.UserLookup.UserIDColumn
			clause := fmt.Sprintf("%s = ?", userCol)
			args := []interface{}{parentID.Bytes()}
			
			// Add tenant isolation if specified
			if config.UserLookup.TenantIsolation && config.UserLookup.TenantColumn != "" {
				clause += fmt.Sprintf(" AND %s = ?", config.UserLookup.TenantColumn)
				// Note: In real implementation, tenant ID would come from context
				args = append(args, "tenant_placeholder")
			}
			
			return clause, args
		},
		
		BuildCountQuery: func(parentID idwrap.IDWrap, config QueryConfigV2) (string, []interface{}) {
			userCol := config.ParentScope.UserLookup.UserIDColumn
			query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s = ?", 
				config.TableName, userCol)
			return query, []interface{}{parentID.Bytes()}
		},
		
		CustomQueryHints: make(map[string]string),
	}
}