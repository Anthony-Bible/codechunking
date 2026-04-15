package zoekt

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/durationpb"

	zoektgrpc "github.com/sourcegraph/zoekt/grpc/protos/zoekt/webserver/v1"
)

// GRPCClient implements the ZoektSearcher outbound port using gRPC.
type GRPCClient struct {
	client zoektgrpc.WebserverServiceClient
	conn   *grpc.ClientConn
	config GRPCClientConfig
}

// GRPCClientConfig holds configuration for the Zoekt gRPC client.
type GRPCClientConfig struct {
	Host    string
	Port    int
	Timeout time.Duration
}

// NewGRPCClient creates a new Zoekt gRPC client connection.
// The connection is established lazily on the first RPC call.
func NewGRPCClient(cfg config.ZoektWebserverConfig) (*GRPCClient, error) {
	conn, err := grpc.NewClient(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Zoekt gRPC client: %w", err)
	}

	return &GRPCClient{
		client: zoektgrpc.NewWebserverServiceClient(conn),
		conn:   conn,
		config: GRPCClientConfig{
			Host:    cfg.Host,
			Port:    cfg.Port,
			Timeout: cfg.Timeout,
		},
	}, nil
}

// Search performs a full-text search query against Zoekt.
func (c *GRPCClient) Search(ctx context.Context, query string, opts outbound.ZoektSearchOptions) (*outbound.ZoektSearchResult, error) {
	startTime := time.Now()

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	q := c.buildSearchQuery(query, opts)

	searchOpts := &zoektgrpc.SearchOptions{
		TotalMaxMatchCount:   int64(opts.MaxTotalResults),
		ShardMaxMatchCount:   int64(opts.MaxTotalResults),
		NumContextLines:      int64(opts.ContextLines),
		ChunkMatches:         opts.ChunkMatches,
		MaxDocDisplayCount:   int64(opts.MaxTotalResults),
		MaxMatchDisplayCount: int64(opts.MaxMatchPerFile),
	}

	if opts.Timeout > 0 {
		searchOpts.MaxWallTime = durationpb.New(opts.Timeout)
	}

	req := &zoektgrpc.SearchRequest{
		Query: q,
		Opts:  searchOpts,
	}

	resp, err := c.client.Search(ctx, req)
	if err != nil {
		slogger.Error(ctx, "Zoekt search failed", slogger.Fields{
			"query": query,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("search failed: %w", err)
	}

	result := &outbound.ZoektSearchResult{
		FileMatches: make([]outbound.ZoektFileMatch, len(resp.GetFiles())),
		TotalCount:  int(resp.GetStats().GetMatchCount()),
		Duration:    time.Since(startTime),
		Stats: outbound.ZoektSearchStats{
			ShardsSearched:  int(resp.GetStats().GetShardsScanned()),
			DocumentCount:   int(resp.GetStats().GetFilesConsidered()),
			IndexBytes:      resp.GetStats().GetIndexBytesLoaded(),
			FilesSearched:   int(resp.GetStats().GetShardFilesConsidered()),
			FilesConsidered: int(resp.GetStats().GetFilesConsidered()),
			BytesSearched:   resp.GetStats().GetContentBytesLoaded(),
		},
	}

	for i, fm := range resp.GetFiles() {
		result.FileMatches[i] = c.convertFileMatch(fm)
	}

	slogger.Debug(ctx, "Zoekt search completed", slogger.Fields{
		"query":           query,
		"result_count":    len(result.FileMatches),
		"duration_ms":     result.Duration.Milliseconds(),
		"shards_searched": result.Stats.ShardsSearched,
	})

	return result, nil
}

// List lists indexed repositories matching a query.
func (c *GRPCClient) List(ctx context.Context, query string, opts outbound.ZoektListOptions) (*outbound.ZoektListResult, error) {
	startTime := time.Now()

	// Apply timeout if set
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Build query: filter by repo name regex if provided, otherwise match all
	var listQuery *zoektgrpc.Q
	if query != "" {
		listQuery = &zoektgrpc.Q{Query: &zoektgrpc.Q_Repo{Repo: &zoektgrpc.Repo{Regexp: query}}}
	} else {
		listQuery = &zoektgrpc.Q{Query: &zoektgrpc.Q_Const{Const: true}}
	}

	req := &zoektgrpc.ListRequest{
		Query: listQuery,
	}

	resp, err := c.client.List(ctx, req)
	if err != nil {
		slogger.Error(ctx, "Zoekt list failed", slogger.Fields{
			"query": query,
			"error": err.Error(),
		})
		return nil, fmt.Errorf("list failed: %w", err)
	}

	repos := resp.GetRepos()

	// Apply MaxResults limit if specified
	if opts.MaxResults > 0 && len(repos) > opts.MaxResults {
		repos = repos[:opts.MaxResults]
	}

	result := &outbound.ZoektListResult{
		Repositories: make([]outbound.ZoektRepoInfo, 0, len(repos)),
		TotalCount:   len(repos),
		Duration:     time.Since(startTime),
	}

	for _, repo := range repos {
		if repo.GetRepository() == nil {
			continue
		}

		repoInfo := outbound.ZoektRepoInfo{
			Name:       repo.GetRepository().GetName(),
			Branch:     "",
			CommitHash: "",
			FilesCount: 0,
			ShardCount: 1,
		}

		if repo.GetIndexMetadata() != nil && repo.GetIndexMetadata().GetIndexTime() != nil {
			repoInfo.IndexTime = repo.GetIndexMetadata().GetIndexTime().AsTime()
		}

		if repo.Repository.Branches != nil && len(repo.GetRepository().GetBranches()) > 0 {
			repoInfo.Branch = repo.GetRepository().GetBranches()[0].GetName()
			repoInfo.CommitHash = repo.GetRepository().GetBranches()[0].GetVersion()
		}

		if repo.GetStats() != nil {
			repoInfo.FilesCount = int(repo.GetStats().GetDocuments())
		}

		result.Repositories = append(result.Repositories, repoInfo)
	}

	slogger.Debug(ctx, "Zoekt list completed", slogger.Fields{
		"query":       query,
		"repo_count":  len(result.Repositories),
		"duration_ms": result.Duration.Milliseconds(),
	})

	return result, nil
}

// CheckHealth checks if the Zoekt webserver is healthy.
func (c *GRPCClient) CheckHealth(ctx context.Context) (*outbound.ZoektHealthStatus, error) {
	healthCtx := ctx
	cancel := func() {}
	if c.config.Timeout > 0 {
		healthCtx, cancel = context.WithTimeout(ctx, c.config.Timeout)
	}
	defer cancel()

	// Use list as health check — Q_Const matches all repos without filtering
	resp, err := c.client.List(healthCtx, &zoektgrpc.ListRequest{
		Query: &zoektgrpc.Q{Query: &zoektgrpc.Q_Const{Const: true}},
	})
	if err != nil {
		slogger.Error(healthCtx, "Zoekt health check failed", slogger.Fields{
			"error": err.Error(),
		})
		return &outbound.ZoektHealthStatus{
			Healthy:    false,
			Version:    "",
			Error:      err.Error(),
			IndexStats: &outbound.ZoektIndexStats{},
		}, nil
	}

	stats := &outbound.ZoektIndexStats{}
	if resp.GetStats() != nil {
		stats.ShardCount = int(resp.GetStats().GetShards())
		stats.DocumentCount = int(resp.GetStats().GetDocuments())
		stats.IndexBytes = resp.GetStats().GetIndexBytes()
		stats.Repositories = int(resp.GetStats().GetRepos())
	}

	return &outbound.ZoektHealthStatus{
		Healthy:    true,
		Version:    "",
		Uptime:     0,
		IndexStats: stats,
	}, nil
}

// Close terminates the gRPC connection.
func (c *GRPCClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// buildSearchQuery builds a Zoekt Q proto from query and options.
func (c *GRPCClient) buildSearchQuery(query string, opts outbound.ZoektSearchOptions) *zoektgrpc.Q {
	q := &zoektgrpc.Q{
		Query: &zoektgrpc.Q_Substring{
			Substring: &zoektgrpc.Substring{
				Pattern:       query,
				CaseSensitive: false,
				Content:       true,
			},
		},
	}

	// TODO: Phase 2 - Add query building for repo, branch, language filters

	return q
}

// convertFileMatch converts a proto FileMatch to domain object.
func (c *GRPCClient) convertFileMatch(fm *zoektgrpc.FileMatch) outbound.ZoektFileMatch {
	lineMatches := make([]outbound.ZoektLineMatch, len(fm.GetLineMatches()))
	for i, lm := range fm.GetLineMatches() {
		lineMatches[i] = outbound.ZoektLineMatch{
			LineNumber:  int(lm.GetLineNumber()),
			LineContent: string(lm.GetLine()),
			Before:      []string{string(lm.GetBefore())},
			After:       []string{string(lm.GetAfter())},
			Score:       lm.GetScore(),
		}
	}

	chunkMatches := make([]outbound.ZoektChunkMatch, len(fm.GetChunkMatches()))
	for i, cm := range fm.GetChunkMatches() {
		startLine := 0
		endLine := 0
		if cm.GetContentStart() != nil {
			startLine = int(cm.GetContentStart().GetLineNumber())
			endLine = startLine + len(cm.GetRanges())
		}

		chunkMatches[i] = outbound.ZoektChunkMatch{
			ChunkID:   "",
			ChunkType: "",
			Context:   string(cm.GetContent()),
			StartLine: startLine,
			EndLine:   endLine,
			Score:     cm.GetScore(),
		}
	}

	branch := ""
	if len(fm.GetBranches()) > 0 {
		branch = fm.GetBranches()[0]
	}

	return outbound.ZoektFileMatch{
		Repository:   fm.GetRepository(),
		FileName:     string(fm.GetFileName()),
		Branch:       branch,
		CommitHash:   fm.GetVersion(),
		Language:     fm.GetLanguage(),
		LineMatches:  lineMatches,
		ChunkMatches: chunkMatches,
		Rank:         int(fm.GetScore()),
		Score:        fm.GetScore(),
	}
}
