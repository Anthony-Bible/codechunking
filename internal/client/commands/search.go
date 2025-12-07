package commands

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/client"
	"context"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Error messages for search command validation.
const (
	errMsgInvalidRepoID = "invalid repository ID: "
)

// NewSearchCmd creates and returns the search command.
// The command performs semantic code search across indexed repositories
// using natural language queries and cosine similarity.
//
// The command requires exactly one argument (the search query) and supports
// extensive filtering and sorting options through flags.
//
// Example usage:
//
//	codechunk search "function to parse JSON" --limit 10 --threshold 0.8
//	codechunk search "error handling" --languages go,python --repo-names myrepo
//	codechunk search "authentication" --repo-ids uuid1,uuid2 --types function
//
// The command writes JSON output to stdout:
//   - Success: {"success": true, "data": {"results": [...], "total": N}}
//   - Failure: {"success": false, "error": {...}}
func NewSearchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search",
		Short: "Perform semantic code search",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				_ = client.WriteError(cmd.OutOrStdout(), errCodeInvalidArgument, errMsgRequiresOneArgument, nil)
				return nil
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return nil
			}

			// Parse --repo-ids flag and validate each UUID.
			// Invalid UUIDs are reported immediately without attempting the search.
			repoIDStrs, _ := cmd.Flags().GetStringSlice("repo-ids")
			var repoIDs []uuid.UUID
			for _, idStr := range repoIDStrs {
				id, err := uuid.Parse(idStr)
				if err != nil {
					_ = client.WriteError(
						cmd.OutOrStdout(),
						errCodeInvalidArgument,
						errMsgInvalidRepoID+idStr,
						nil,
					)
					return nil
				}
				repoIDs = append(repoIDs, id)
			}

			// Build search request from query argument and filter flags.
			// All filter parameters are optional and applied as AND conditions.
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			repoNames, _ := cmd.Flags().GetStringSlice("repo-names")
			languages, _ := cmd.Flags().GetStringSlice("languages")
			fileTypes, _ := cmd.Flags().GetStringSlice("file-types")
			threshold, _ := cmd.Flags().GetFloat64("threshold")
			sort, _ := cmd.Flags().GetString("sort")
			types, _ := cmd.Flags().GetStringSlice("types")
			entityName, _ := cmd.Flags().GetString("entity-name")
			visibility, _ := cmd.Flags().GetStringSlice("visibility")

			req := dto.SearchRequestDTO{
				Query:               args[0],
				Limit:               limit,
				Offset:              offset,
				RepositoryIDs:       repoIDs,
				RepositoryNames:     repoNames,
				Languages:           languages,
				FileTypes:           fileTypes,
				SimilarityThreshold: threshold,
				Sort:                sort,
				Types:               types,
				EntityName:          entityName,
				Visibility:          visibility,
			}

			c, ok := createClientFromFlags(cmd, cmd.OutOrStdout())
			if !ok {
				return nil
			}

			timeout, _ := cmd.Flags().GetDuration("timeout")
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()

			result, err := c.Search(ctx, req)
			if err != nil {
				code := determineErrorCode(err)
				_ = client.WriteError(cmd.OutOrStdout(), code, err.Error(), nil)
				return nil
			}

			return client.WriteSuccess(cmd.OutOrStdout(), result)
		},
	}

	// Register search-specific flags with detailed descriptions.
	cmd.Flags().IntP("limit", "l", dto.DefaultSearchLimit, "Maximum number of results to return")
	cmd.Flags().Int("offset", 0, "Number of results to skip for pagination")
	cmd.Flags().StringSlice("repo-ids", nil, "Filter by repository UUIDs (comma-separated)")
	cmd.Flags().StringSlice("repo-names", nil, "Filter by repository names (comma-separated)")
	cmd.Flags().StringSlice("languages", nil, "Filter by programming languages (e.g., go,python,javascript)")
	cmd.Flags().StringSlice("file-types", nil, "Filter by file extensions (e.g., .go,.py,.js)")
	cmd.Flags().
		Float64P("threshold", "t", dto.DefaultSimilarityThreshold, "Minimum cosine similarity threshold (0.0-1.0)")
	cmd.Flags().StringP("sort", "s", dto.DefaultSearchSort, "Sort order for results (e.g., relevance, date)")
	cmd.Flags().StringSlice("types", nil, "Filter by semantic types (e.g., function,class,interface)")
	cmd.Flags().String("entity-name", "", "Filter by specific entity name (exact match)")
	cmd.Flags().StringSlice("visibility", nil, "Filter by visibility scope (e.g., public,private)")

	return cmd
}
