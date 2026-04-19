package zoekt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseIndexOutput(t *testing.T) {
	indexer := &Indexer{}

	tests := []struct {
		name          string
		output        string
		wantShards    int
		wantFiles     int
		wantBytes     int64
		wantPathCount int
	}{
		{
			name: "single shard",
			output: `2026/04/18 18:59:08 failed to open submodule repository: wiki, submodule not initialized
2026/04/18 18:59:08 cat-file batch disabled via ZOEKT_DISABLE_CATFILE_BATCH, using go-git
2026/04/18 18:59:08 attempting to index 701 total files (0 via cat-file, 701 via go-git)
2026/04/18 18:59:09 finished shard /tmp/test-zoekt-index/github.com%2FAnthony-Bible%2Fcodechunking_v16.00000.zoekt: 29941290 index bytes (overhead 2.8), 701 files processed`,
			wantShards:    1,
			wantFiles:     701,
			wantBytes:     29941290,
			wantPathCount: 1,
		},
		{
			name: "multiple shards",
			output: `2026/04/18 19:00:00 finished shard /tmp/index/repo_v1.00000.zoekt: 1000 index bytes (overhead 1.5), 100 files processed
2026/04/18 19:00:01 finished shard /tmp/index/repo_v1.00001.zoekt: 2000 index bytes (overhead 1.5), 200 files processed`,
			wantShards:    2,
			wantFiles:     300,
			wantBytes:     3000,
			wantPathCount: 2,
		},
		{
			name:          "empty output",
			output:        "",
			wantShards:    0,
			wantFiles:     0,
			wantBytes:     0,
			wantPathCount: 0,
		},
		{
			name:          "no shard lines",
			output:        "2026/04/18 19:00:00 attempting to index 10 files",
			wantShards:    0,
			wantFiles:     0,
			wantBytes:     0,
			wantPathCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := indexer.parseIndexOutput(tt.output)
			assert.Equal(t, tt.wantShards, result.ShardCount, "ShardCount")
			assert.Equal(t, tt.wantFiles, result.FileCount, "FileCount")
			assert.Equal(t, tt.wantBytes, result.BytesIndexed, "BytesIndexed")
			assert.Len(t, result.ShardPaths, tt.wantPathCount, "ShardPaths length")
		})
	}
}
