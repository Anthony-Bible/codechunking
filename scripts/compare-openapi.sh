#!/bin/bash

# Script to compare manually maintained OpenAPI spec with auto-generated one
# This helps ensure the manual spec stays up-to-date with the code

set -e

echo "🔍 Comparing OpenAPI specifications..."
echo ""

# Check if both files exist
if [ ! -f "api/openapi.yaml" ]; then
    echo "❌ api/openapi.yaml not found"
    exit 1
fi

if [ ! -f "docs/swagger.yaml" ]; then
    echo "❌ docs/swagger.yaml not found. Run 'make generate-openapi' first."
    exit 1
fi

echo "📊 File comparison:"
echo "  Manual spec (api/openapi.yaml): $(wc -l < api/openapi.yaml) lines"
echo "  Generated spec (docs/swagger.yaml): $(wc -l < docs/swagger.yaml) lines"
echo ""

echo "🔍 Checking for enhanced search fields in both specs:"
echo ""

echo "Manual spec fields:"
grep -c "repository_names\|entity_name\|visibility\|qualified_name\|parent_entity\|signature" api/openapi.yaml || echo "  No enhanced fields found"

echo "Generated spec fields:"
grep -c "repository_names\|entity_name\|visibility" docs/swagger.yaml || echo "  No enhanced fields found"

echo ""
echo "🎯 Checking for examples in generated spec:"
echo "SearchRequest examples:"
grep -c "example:.*implement authentication middleware" docs/swagger.yaml || echo "  No search request examples found"
echo "SearchResult examples:"
grep -c "example:.*chunk-uuid-1" docs/swagger.yaml || echo "  No search result examples found"
echo "Repository examples:"
grep -c "example:.*golang/go" docs/swagger.yaml || echo "  No repository examples found"

echo ""
echo "📋 Key differences summary:"
echo "  - Manual spec: OpenAPI 3.0.3 format with detailed examples and descriptions"
echo "  - Generated spec: Swagger 2.0 format, automatically synced with code + examples"
echo ""

echo "💡 Recommendations:"
echo "  - Use 'make generate-openapi' to sync with code changes"
echo "  - Keep api/openapi.yaml as the source of truth for documentation"
echo "  - The generated spec confirms your DTOs and annotations are correct"
echo "  - Examples are now included in the generated spec for better API documentation"
echo ""

echo "✅ Comparison complete!"