package outbound

import (
	"codechunking/internal/domain/valueobject"
	"context"
	"errors"
	"time"
)

// Authentication timeout constants.
const (
	DefaultAuthTimeoutSeconds = 30
)

// GitClient defines the interface for git operations.
type GitClient interface {
	Clone(ctx context.Context, repoURL, targetPath string) error
	GetCommitHash(ctx context.Context, repoPath string) (string, error)
	GetBranch(ctx context.Context, repoPath string) (string, error)
}

// EnhancedGitClient extends GitClient with shallow cloning and monitoring capabilities.
type EnhancedGitClient interface {
	GitClient

	// CloneWithOptions performs git clone with configurable options.
	CloneWithOptions(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
	) (*CloneResult, error)

	// GetRepositoryInfo retrieves repository metadata without cloning.
	GetRepositoryInfo(ctx context.Context, repoURL string) (*RepositoryInfo, error)

	// ValidateRepository checks if a repository URL is valid and accessible.
	ValidateRepository(ctx context.Context, repoURL string) (bool, error)

	// EstimateCloneTime estimates how long a clone operation will take.
	EstimateCloneTime(ctx context.Context, repoURL string, opts valueobject.CloneOptions) (*CloneEstimation, error)

	// GetCloneProgress returns the progress of an ongoing clone operation.
	GetCloneProgress(ctx context.Context, operationID string) (*CloneProgress, error)

	// CancelClone cancels an ongoing clone operation.
	CancelClone(ctx context.Context, operationID string) error
}

// CloneResult contains the results of a successful clone operation.
type CloneResult struct {
	OperationID    string        `json:"operation_id"`
	CommitHash     string        `json:"commit_hash"`
	BranchName     string        `json:"branch_name"`
	CloneTime      time.Duration `json:"clone_time"`
	RepositorySize int64         `json:"repository_size"`
	FileCount      int           `json:"file_count"`
	CloneDepth     int           `json:"clone_depth"`
}

// RepositoryInfo contains metadata about a repository.
type RepositoryInfo struct {
	DefaultBranch    string           `json:"default_branch"`
	EstimatedSize    int64            `json:"estimated_size"`
	CommitCount      int              `json:"commit_count"`
	Branches         []string         `json:"branches"`
	LastCommitDate   time.Time        `json:"last_commit_date"`
	IsPrivate        bool             `json:"is_private"`
	Languages        map[string]int64 `json:"languages"`
	HasSubmodules    bool             `json:"has_submodules"`
	RecommendedDepth int              `json:"recommended_depth"`
}

// CloneEstimation provides estimates for clone operations.
type CloneEstimation struct {
	EstimatedDuration time.Duration `json:"estimated_duration"`
	EstimatedSize     int64         `json:"estimated_size"`
	Confidence        float64       `json:"confidence"` // 0.0 to 1.0
	RecommendedDepth  int           `json:"recommended_depth"`
	UsesShallowClone  bool          `json:"uses_shallow_clone"`
}

// CloneProgress tracks the progress of a clone operation.
type CloneProgress struct {
	OperationID        string        `json:"operation_id"`
	Status             string        `json:"status"`     // "preparing", "cloning", "completed", "failed", "cancelled"
	Percentage         float64       `json:"percentage"` // 0.0 to 100.0
	BytesReceived      int64         `json:"bytes_received"`
	TotalBytes         int64         `json:"total_bytes"`
	FilesProcessed     int           `json:"files_processed"`
	CurrentFile        string        `json:"current_file"`
	StartTime          time.Time     `json:"start_time"`
	EstimatedRemaining time.Duration `json:"estimated_remaining"`
}

// AuthenticatedGitClient extends EnhancedGitClient with authentication capabilities.
type AuthenticatedGitClient interface {
	EnhancedGitClient

	// CloneWithSSHAuth performs git clone using SSH key authentication.
	CloneWithSSHAuth(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *SSHAuthConfig,
	) (*AuthenticatedCloneResult, error)

	// CloneWithTokenAuth performs git clone using token-based authentication.
	CloneWithTokenAuth(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
	) (*AuthenticatedCloneResult, error)

	// ValidateSSHKey validates an SSH key for authentication.
	ValidateSSHKey(ctx context.Context, authConfig *SSHAuthConfig) (bool, error)

	// ValidateToken validates a token for authentication.
	ValidateToken(ctx context.Context, authConfig *TokenAuthConfig) (bool, error)

	// DiscoverSSHKeys discovers available SSH keys in default locations.
	DiscoverSSHKeys(ctx context.Context, homeDir string) (*SSHKeyInfo, error)

	// TestRepositoryAccess tests if repository is accessible with given credentials.
	TestRepositoryAccess(ctx context.Context, repoURL string, authConfig AuthConfig) (bool, error)

	// ValidateTokenScopes validates if a token has the required scopes for an operation.
	ValidateTokenScopes(ctx context.Context, authConfig *TokenAuthConfig, requiredScopes []string) (bool, error)

	// Credential loading methods
	LoadCredentialsFromEnvironment(ctx context.Context, repoURL string) (AuthConfig, error)
	LoadCredentialsFromFile(ctx context.Context, configPath, repoURL string) (AuthConfig, error)
	ResolveCredentials(
		ctx context.Context,
		repoURL string,
		explicit AuthConfig,
		configPath string,
	) (AuthConfig, string, error)

	// Secure credential storage methods
	StoreCredentialSecurely(ctx context.Context, credType, data, method string) (string, error)
	RetrieveCredentialSecurely(ctx context.Context, credentialID string) (string, error)
	IsCredentialEncrypted(ctx context.Context, credentialID string) (bool, error)
	DeleteCredentialSecurely(ctx context.Context, credentialID string) error

	// Advanced authentication methods
	CloneWithCachedAuth(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig AuthConfig,
		cacheConfig *CredentialCacheConfig,
	) (*AuthenticatedCloneResult, bool, error)
	RotateCredentials(
		ctx context.Context,
		authConfig *TokenAuthConfig,
		rotationConfig *CredentialRotationConfig,
	) (*TokenAuthConfig, bool, error)

	// Retry and progress methods
	CloneWithTokenAuthAndRetry(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
		retryConfig *AuthRetryConfig,
	) (*AuthenticatedCloneResult, int, error)
	CloneWithSSHAuthAndProgress(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *SSHAuthConfig,
		progressConfig *ProgressConfig,
	) (*AuthenticatedCloneResult, error)
	CloneWithTokenAuthAndProgress(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
		progressConfig *ProgressConfig,
	) (*AuthenticatedCloneResult, error)

	// Security methods
	CloneWithSSHAuthSecure(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *SSHAuthConfig,
		securityConfig *SecurityAuditConfig,
	) (string, error)
	CloneWithTokenAuthSecure(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
		securityConfig *SecurityAuditConfig,
	) (string, error)
	VerifyMemoryProtection(ctx context.Context, authConfig AuthConfig) (bool, error)
	ValidateCredentialsSecurely(ctx context.Context, authConfig AuthConfig) (bool, string, error)
	LoadCredentialsIntoSecureMemory(ctx context.Context, authConfig AuthConfig) (string, error)
	VerifySecureMemoryAccess(ctx context.Context, memoryHandle string) (bool, error)
	WipeSecureMemory(ctx context.Context, memoryHandle string) error
	SanitizeAuthenticationInputs(ctx context.Context, inputs map[string]string) (map[string]string, error)

	// Audit methods
	CloneWithSSHAuthAndAudit(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *SSHAuthConfig,
		auditConfig *AuditConfig,
	) (string, error)
	CloneWithTokenAuthAndAudit(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
		auditConfig *AuditConfig,
	) (string, error)

	// Cleanup methods
	CloneWithSSHAuthAndCleanup(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *SSHAuthConfig,
		cleanupConfig *CleanupConfig,
	) (*CleanupReport, error)
	CloneWithTokenAuthAndCleanup(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig *TokenAuthConfig,
		cleanupConfig *CleanupConfig,
	) (*CleanupReport, error)
	VerifyCredentialCleanup(ctx context.Context, credentialType, cleanupID string) (bool, error)
}

// AuthConfig is a generic interface for authentication configurations.
type AuthConfig interface {
	GetAuthType() string
	Validate() error
	IsSecure() bool
}

// SSHAuthConfig holds configuration for SSH key authentication.
type SSHAuthConfig struct {
	KeyPath            string        `json:"key_path,omitempty"`    // Path to private key file
	Passphrase         string        `json:"-"`                     // Key passphrase (never serialized)
	UseSSHAgent        bool          `json:"use_ssh_agent"`         // Whether to use SSH agent
	Timeout            time.Duration `json:"timeout"`               // Authentication timeout
	StrictHostKeyCheck bool          `json:"strict_host_key_check"` // Whether to verify host keys
}

// GetAuthType returns the authentication type.
func (s *SSHAuthConfig) GetAuthType() string {
	return "ssh"
}

// Validate validates the SSH authentication configuration.
func (s *SSHAuthConfig) Validate() error {
	if s.KeyPath == "" && !s.UseSSHAgent {
		return errors.New("either key_path must be specified or use_ssh_agent must be true")
	}
	if s.Timeout <= 0 {
		s.Timeout = DefaultAuthTimeoutSeconds * time.Second // default timeout
	}
	return nil
}

// IsSecure returns true as SSH key authentication is considered secure.
func (s *SSHAuthConfig) IsSecure() bool {
	return true
}

// TokenAuthConfig holds configuration for token-based authentication.
type TokenAuthConfig struct {
	Token     string        `json:"-"`                  // Access token (never serialized)
	TokenType string        `json:"token_type"`         // Type: "pat", "oauth", "github_app"
	Provider  string        `json:"provider"`           // Provider: "github", "gitlab", "bitbucket"
	Username  string        `json:"username,omitempty"` // Username for PAT auth
	Timeout   time.Duration `json:"timeout"`            // Authentication timeout
	Scopes    []string      `json:"scopes,omitempty"`   // Required scopes for the token
}

// GetAuthType returns the authentication type.
func (t *TokenAuthConfig) GetAuthType() string {
	return "token"
}

// Validate validates the token authentication configuration.
func (t *TokenAuthConfig) Validate() error {
	if t.Token == "" {
		return errors.New("token is required")
	}
	if t.Provider == "" {
		return errors.New("provider is required")
	}
	if t.TokenType == "" {
		t.TokenType = "pat" // default to Personal Access Token
	}
	if t.Timeout <= 0 {
		t.Timeout = DefaultAuthTimeoutSeconds * time.Second // default timeout
	}
	return nil
}

// IsSecure returns true as token authentication is considered secure.
func (t *TokenAuthConfig) IsSecure() bool {
	return true
}

// AuthenticatedCloneResult extends CloneResult with authentication information.
type AuthenticatedCloneResult struct {
	*CloneResult

	AuthMethod   string   `json:"auth_method"`            // "ssh" or "token"
	AuthProvider string   `json:"auth_provider"`          // e.g., "github", "gitlab"
	UsedSSHAgent bool     `json:"used_ssh_agent"`         // Whether SSH agent was used
	TokenScopes  []string `json:"token_scopes,omitempty"` // Scopes of the used token
}

// SSHKeyInfo contains information about discovered SSH keys.
type SSHKeyInfo struct {
	KeyPath  string `json:"key_path"`
	KeyType  string `json:"key_type"` // "rsa", "ed25519", "ecdsa"
	IsValid  bool   `json:"is_valid"`
	HasAgent bool   `json:"has_agent"` // Whether key is loaded in SSH agent
	KeySize  int    `json:"key_size"`  // Key size in bits
}

// CredentialCacheConfig holds configuration for credential caching.
type CredentialCacheConfig struct {
	Duration string `json:"duration"` // Cache duration (e.g., "5m", "1h")
	Enabled  bool   `json:"enabled"`  // Whether caching is enabled
}

// CredentialRotationConfig holds configuration for automatic credential rotation.
type CredentialRotationConfig struct {
	Enabled           bool   `json:"enabled"`            // Whether rotation is enabled
	RotationThreshold string `json:"rotation_threshold"` // Threshold for rotation (e.g., "24h")
	RefreshToken      string `json:"-"`                  // Refresh token for OAuth (never serialized)
}

// AuthRetryConfig holds configuration for authentication retry logic.
type AuthRetryConfig struct {
	MaxAttempts     int           `json:"max_attempts"`     // Maximum number of retry attempts
	BackoffDuration time.Duration `json:"backoff_duration"` // Initial backoff duration
	RetryableErrors []string      `json:"retryable_errors"` // List of retryable error types
}

// ProgressConfig holds configuration for progress tracking.
type ProgressConfig struct {
	Enabled  bool          `json:"enabled"`  // Whether progress tracking is enabled
	Interval time.Duration `json:"interval"` // Progress update interval
}

// SecurityAuditConfig holds configuration for security auditing.
type SecurityAuditConfig struct {
	LogSanitization     bool `json:"log_sanitization"`     // Whether to sanitize logs
	MemoryProtection    bool `json:"memory_protection"`    // Whether to enable memory protection
	CredentialRedaction bool `json:"credential_redaction"` // Whether to redact credentials in logs
}

// AuditConfig holds configuration for audit logging.
type AuditConfig struct {
	Enabled           bool   `json:"enabled"`            // Whether audit logging is enabled
	LogLevel          string `json:"log_level"`          // Log level for audit events
	RedactCredentials bool   `json:"redact_credentials"` // Whether to redact credentials
	LogDestination    string `json:"log_destination"`    // Where to send audit logs
}

// CleanupConfig holds configuration for credential cleanup.
type CleanupConfig struct {
	VerifyCleanup  bool `json:"verify_cleanup"`  // Whether to verify cleanup
	SecureWipe     bool `json:"secure_wipe"`     // Whether to securely wipe data
	CleanupTimeout int  `json:"cleanup_timeout"` // Cleanup timeout in seconds
}

// CleanupReport contains information about credential cleanup operations.
type CleanupReport struct {
	CleanupID        string `json:"cleanup_id"`         // Unique cleanup operation ID
	CredentialsWiped bool   `json:"credentials_wiped"`  // Whether credentials were wiped
	MemoryCleared    bool   `json:"memory_cleared"`     // Whether memory was cleared
	TempFilesRemoved bool   `json:"temp_files_removed"` // Whether temp files were removed
}

// GitOperationError represents an error from a git operation.
type GitOperationError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Cause   error  `json:"cause,omitempty"`
}

// Error implements the error interface.
func (e *GitOperationError) Error() string {
	if e.Cause != nil {
		return "git operation failed (" + e.Type + "): " + e.Message + ": " + e.Cause.Error()
	}
	return "git operation failed (" + e.Type + "): " + e.Message
}

// Unwrap returns the underlying cause error.
func (e *GitOperationError) Unwrap() error {
	return e.Cause
}

// CodeParser defines the interface for parsing code.
type CodeParser interface {
	ParseDirectory(ctx context.Context, dirPath string, config CodeParsingConfig) ([]CodeChunk, error)
}

// CodeParsingConfig holds configuration for code parsing.
type CodeParsingConfig struct {
	ChunkSizeBytes   int
	MaxFileSizeBytes int64
	FileFilters      []string
	IncludeTests     bool
	ExcludeVendor    bool
}

// CodeChunk represents a parsed code chunk.
type CodeChunk struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"file_path"`
	StartLine int       `json:"start_line"`
	EndLine   int       `json:"end_line"`
	Content   string    `json:"content"`
	Language  string    `json:"language"`
	Size      int       `json:"size"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

// ====================== CACHING INTERFACES ======================

// CacheStrategy represents different cache management strategies.
type CacheStrategy int

const (
	// CacheStrategyLRU uses Least Recently Used cache eviction policy.
	CacheStrategyLRU CacheStrategy = iota
	// CacheStrategyTTL uses Time To Live based cache expiration.
	CacheStrategyTTL
	// CacheStrategyCommitHash uses commit hash-based cache validation.
	CacheStrategyCommitHash
)

// CacheStorageType represents different cache storage mechanisms.
type CacheStorageType int

const (
	// CacheStorageFilesystem stores cache entries on the filesystem.
	CacheStorageFilesystem CacheStorageType = iota
	// CacheStorageMemoryMapped uses memory-mapped file storage for cache entries.
	CacheStorageMemoryMapped
	// CacheStorageHybrid combines filesystem and memory-mapped storage.
	CacheStorageHybrid
)

// CachedGitClient extends AuthenticatedGitClient with caching capabilities.
type CachedGitClient interface {
	AuthenticatedGitClient

	// Clone with caching support
	CloneWithCache(
		ctx context.Context,
		repoURL, targetPath string,
		opts valueobject.CloneOptions,
		authConfig AuthConfig,
		cacheConfig *CloneCacheConfig,
	) (*CachedCloneResult, bool, error)

	// Cache management operations
	GetCacheEntry(ctx context.Context, repoURL, branch string, authConfig AuthConfig) (*CacheEntry, bool)
	InvalidateCacheForCommitChange(ctx context.Context, repoURL, oldHash, newHash string) error
	SimulateTimePassage(duration time.Duration) error // For testing
	ClearCache(ctx context.Context, pattern string) error
	GetCacheStatistics(ctx context.Context) (*CacheStatistics, error)
}

// CacheKeyGenerator generates cache keys for different scenarios.
type CacheKeyGenerator interface {
	GenerateCacheKey(repoURL, branch string, authConfig AuthConfig, config *CloneCacheConfig) (string, error)
	ValidateCacheKey(key string) error
	ParseCacheKey(key string) (*CacheKeyComponents, error)
}

// CacheStorageManager manages different storage mechanisms for cache.
type CacheStorageManager interface {
	Initialize(config *CacheStorageConfig) error
	CreateCacheEntry(key, repoPath string, sizeBytes int64) error
	ReadCacheEntry(key string) (*StoredCacheEntry, error)
	DeleteCacheEntry(key string) error
	CompressCacheEntry(key string) error
	MemoryMapEntry(key string) error
	PromoteToHotTier(key string) error
	DemoteToColTier(key string) error
	Cleanup() error
}

// CloneCacheConfig holds configuration for clone caching.
type CloneCacheConfig struct {
	Enabled                  bool          `json:"enabled"`
	TTL                      time.Duration `json:"ttl"`
	MaxSizeGB                int64         `json:"max_size_gb"`
	Strategy                 CacheStrategy `json:"strategy"`
	BaseDir                  string        `json:"base_dir"`
	BranchSpecific           bool          `json:"branch_specific"`
	AuthAgnostic             bool          `json:"auth_agnostic"`
	ShallowFullCompatibility bool          `json:"shallow_full_compatibility"`
	CommitHashTracking       bool          `json:"commit_hash_tracking"`
	CompressionEnabled       bool          `json:"compression_enabled"`
	EncryptionEnabled        bool          `json:"encryption_enabled"`
}

// CacheStorageConfig holds configuration for cache storage mechanisms.
type CacheStorageConfig struct {
	Type           CacheStorageType `json:"type"`
	BaseDir        string           `json:"base_dir"`
	MaxSizeGB      int64            `json:"max_size_gb"`
	HotTierSizeGB  int64            `json:"hot_tier_size_gb,omitempty"`
	ColdTierSizeGB int64            `json:"cold_tier_size_gb,omitempty"`
	UseMemoryMap   bool             `json:"use_memory_map"`
	Compression    bool             `json:"compression"`
}

// CachedCloneResult extends AuthenticatedCloneResult with cache information.
type CachedCloneResult struct {
	*AuthenticatedCloneResult

	CacheHit     bool          `json:"cache_hit"`
	CacheKey     string        `json:"cache_key"`
	CacheMetrics *CacheMetrics `json:"cache_metrics"`
}

// CacheMetrics contains performance and usage metrics for cache operations.
type CacheMetrics struct {
	PerformanceGain   float64       `json:"performance_gain"`   // Speed improvement ratio
	SpaceSaved        int64         `json:"space_saved"`        // Bytes saved by caching
	CacheHitRatio     float64       `json:"cache_hit_ratio"`    // Overall hit ratio
	AverageHitTime    time.Duration `json:"average_hit_time"`   // Average cache hit time
	AverageMissTime   time.Duration `json:"average_miss_time"`  // Average cache miss time
	CompressionRatio  float64       `json:"compression_ratio"`  // Compression effectiveness
	StorageEfficiency float64       `json:"storage_efficiency"` // Storage space efficiency
}

// CacheEntry represents a single cache entry.
type CacheEntry struct {
	Key                string    `json:"key"`
	RepoURL            string    `json:"repo_url"`
	Branch             string    `json:"branch"`
	CommitHash         string    `json:"commit_hash"`
	CachedPath         string    `json:"cached_path"`
	CreatedAt          time.Time `json:"created_at"`
	LastAccessedAt     time.Time `json:"last_accessed_at"`
	AccessCount        int64     `json:"access_count"`
	SizeBytes          int64     `json:"size_bytes"`
	IsValid            bool      `json:"is_valid"`
	InvalidationReason string    `json:"invalidation_reason,omitempty"`
	CompressionRatio   float64   `json:"compression_ratio"`
	EncryptionEnabled  bool      `json:"encryption_enabled"`
}

// StoredCacheEntry represents the stored form of a cache entry.
type StoredCacheEntry struct {
	*CacheEntry

	StorageType CacheStorageType `json:"storage_type"`
	IsMapped    bool             `json:"is_mapped"`
	TierLevel   int              `json:"tier_level"` // 0=hot, 1=cold
}

// CacheKeyComponents represents the parsed components of a cache key.
type CacheKeyComponents struct {
	RepoURL  string `json:"repo_url"`
	Branch   string `json:"branch"`
	AuthHash string `json:"auth_hash,omitempty"`
	OptsHash string `json:"opts_hash"`
	Version  int    `json:"version"`
}

// CacheStatistics represents overall cache performance statistics.
type CacheStatistics struct {
	TotalEntries       int64         `json:"total_entries"`
	TotalSizeBytes     int64         `json:"total_size_bytes"`
	HitCount           int64         `json:"hit_count"`
	MissCount          int64         `json:"miss_count"`
	EvictionCount      int64         `json:"eviction_count"`
	AverageHitTime     time.Duration `json:"average_hit_time"`
	AverageMissTime    time.Duration `json:"average_miss_time"`
	StorageUtilization float64       `json:"storage_utilization"`
	OldestEntry        time.Time     `json:"oldest_entry"`
	NewestEntry        time.Time     `json:"newest_entry"`
}

// ====================== INCREMENTAL UPDATE INTERFACES ======================

// UpdateStrategy represents different update strategies.
type UpdateStrategy int

const (
	// UpdateStrategyIncrementalFetch fetches only new commits incrementally.
	UpdateStrategyIncrementalFetch UpdateStrategy = iota
	// UpdateStrategyFullClone performs a complete clone operation.
	UpdateStrategyFullClone
	// UpdateStrategySmartIncremental intelligently chooses between incremental and full clone.
	UpdateStrategySmartIncremental
)

// BranchUpdateStrategy represents different branch update strategies.
type BranchUpdateStrategy int

const (
	// BranchUpdateStrategySame keeps the same branch as cached.
	BranchUpdateStrategySame BranchUpdateStrategy = iota
	// BranchUpdateStrategySwitch switches to a different existing branch.
	BranchUpdateStrategySwitch
	// BranchUpdateStrategyCreate creates a new branch from current state.
	BranchUpdateStrategyCreate
	// BranchUpdateStrategyMerge merges changes from another branch.
	BranchUpdateStrategyMerge
	// BranchUpdateStrategyForceUpdate forcefully updates to target branch state.
	BranchUpdateStrategyForceUpdate
)

// ConflictType represents different types of conflicts.
type ConflictType int

const (
	// ConflictTypeMerge represents merge conflicts in text files.
	ConflictTypeMerge ConflictType = iota
	// ConflictTypeBinary represents conflicts in binary files.
	ConflictTypeBinary
	// ConflictTypeDeleted represents conflicts with deleted files.
	ConflictTypeDeleted
	// ConflictTypeSubmodule represents submodule conflicts.
	ConflictTypeSubmodule
	// ConflictTypePermissions represents file permission conflicts.
	ConflictTypePermissions
)

// ConflictResolutionStrategy represents different conflict resolution strategies.
type ConflictResolutionStrategy int

const (
	// ConflictResolutionAutomatic automatically resolves conflicts when possible.
	ConflictResolutionAutomatic ConflictResolutionStrategy = iota
	// ConflictResolutionManual requires manual intervention for conflict resolution.
	ConflictResolutionManual
	// ConflictResolutionPreferRemote automatically chooses remote changes in conflicts.
	ConflictResolutionPreferRemote
	// ConflictResolutionPreserveLocal automatically chooses local changes in conflicts.
	ConflictResolutionPreserveLocal
	// ConflictResolutionFallbackFullClone falls back to full clone when conflicts are complex.
	ConflictResolutionFallbackFullClone
)

// IncrementalUpdateHandler manages incremental updates and conflict resolution.
type IncrementalUpdateHandler interface {
	// Decision making
	DecideUpdateStrategy(ctx context.Context, context *UpdateDecisionContext) (*UpdateDecision, error)
	AnalyzeCommitDelta(ctx context.Context, baseCommit, targetCommit string) (*CommitDelta, error)
	AnalyzeBranchUpdate(ctx context.Context, context *BranchUpdateContext) (*BranchUpdatePlan, error)

	// Conflict resolution
	ResolveConflict(ctx context.Context, conflict *ConflictContext) (*ConflictResolution, error)

	// Performance analysis
	ComparePerformance(ctx context.Context, context *PerformanceContext) (*PerformanceComparison, error)

	// Execution
	ExecuteIncrementalUpdate(ctx context.Context, config *IncrementalUpdateConfig) (*UpdateResult, error)
	ExecuteFullUpdate(ctx context.Context, config *FullUpdateConfig) (*UpdateResult, error)
}

// UpdateDecisionContext provides context for update strategy decisions.
type UpdateDecisionContext struct {
	RepoURL          string        `json:"repo_url"`
	CachedCommitHash string        `json:"cached_commit_hash"`
	RemoteCommitHash string        `json:"remote_commit_hash"`
	CacheAge         time.Duration `json:"cache_age"`
	RepositorySizeMB int64         `json:"repository_size_mb"`
	CommitsBehind    int           `json:"commits_behind"`
}

// BranchUpdateContext provides context for branch update decisions.
type BranchUpdateContext struct {
	CachedBranch    string `json:"cached_branch"`
	RequestedBranch string `json:"requested_branch"`
	BranchExists    bool   `json:"branch_exists"`
	BranchDiverged  bool   `json:"branch_diverged"`
}

// ConflictContext provides context for conflict resolution.
type ConflictContext struct {
	Type     ConflictType               `json:"type"`
	Files    []string                   `json:"files"`
	Strategy ConflictResolutionStrategy `json:"strategy"`
}

// PerformanceContext provides context for performance comparison.
type PerformanceContext struct {
	RepositorySize   int64 `json:"repository_size"`
	CommitsBehind    int   `json:"commits_behind"`
	FilesChanged     int   `json:"files_changed"`
	NetworkSpeedMbps int   `json:"network_speed_mbps"`
}

// UpdateDecision contains the result of update strategy decision.
type UpdateDecision struct {
	Strategy               UpdateStrategy `json:"strategy"`
	Reason                 string         `json:"reason"`
	EstimatedTimeSavings   float64        `json:"estimated_time_savings"`
	EstimatedSizeReduction int64          `json:"estimated_size_reduction"`
}

// CommitDelta contains analysis of commits between base and target.
type CommitDelta struct {
	Commits     []string `json:"commits"`
	FileChanges int      `json:"file_changes"`
	Conflicts   []string `json:"conflicts"`
	SizeDelta   int64    `json:"size_delta"`
}

// BranchUpdatePlan contains strategy for branch updates.
type BranchUpdatePlan struct {
	Strategy           BranchUpdateStrategy `json:"strategy"`
	RequiredActions    []string             `json:"required_actions"`
	RequiresFullUpdate bool                 `json:"requires_full_update"`
	EstimatedDuration  time.Duration        `json:"estimated_duration"`
}

// ConflictResolution contains result of conflict resolution.
type ConflictResolution struct {
	Resolved          bool     `json:"resolved"`
	Actions           []string `json:"actions"`
	RequiresFullClone bool     `json:"requires_full_clone"`
	BackupCreated     bool     `json:"backup_created"`
}

// PerformanceComparison contains performance comparison results.
type PerformanceComparison struct {
	IncrementalTime     time.Duration  `json:"incremental_time"`
	FullCloneTime       time.Duration  `json:"full_clone_time"`
	SpeedupRatio        float64        `json:"speedup_ratio"`
	BandwidthSaved      int64          `json:"bandwidth_saved"`
	RecommendedStrategy UpdateStrategy `json:"recommended_strategy"`
}

// UpdateResult contains the result of an update operation.
type UpdateResult struct {
	Success           bool           `json:"success"`
	Strategy          UpdateStrategy `json:"strategy"`
	Duration          time.Duration  `json:"duration"`
	BytesTransferred  int64          `json:"bytes_transferred"`
	CommitsProcessed  int            `json:"commits_processed"`
	ConflictsResolved int            `json:"conflicts_resolved"`
	FinalCommitHash   string         `json:"final_commit_hash"`
}

// IncrementalUpdateConfig holds configuration for incremental updates.
type IncrementalUpdateConfig struct {
	RepoURL              string                     `json:"repo_url"`
	CachedPath           string                     `json:"cached_path"`
	TargetCommit         string                     `json:"target_commit"`
	ConflictResolution   ConflictResolutionStrategy `json:"conflict_resolution"`
	MaxCommitsBehind     int                        `json:"max_commits_behind"`
	MaxFilesChanged      int                        `json:"max_files_changed"`
	TimeoutDuration      time.Duration              `json:"timeout_duration"`
	CreateBackup         bool                       `json:"create_backup"`
	PreserveDirtyChanges bool                       `json:"preserve_dirty_changes"`
}

// FullUpdateConfig holds configuration for full updates.
type FullUpdateConfig struct {
	RepoURL         string                   `json:"repo_url"`
	TargetPath      string                   `json:"target_path"`
	CloneOptions    valueobject.CloneOptions `json:"clone_options"`
	BackupOldCache  bool                     `json:"backup_old_cache"`
	TimeoutDuration time.Duration            `json:"timeout_duration"`
}
