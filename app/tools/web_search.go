//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ollama/ollama/internal/websearch"
)

const defaultWebSearchResults = 5

// SearchResult represents a single result from the local single-query search helper.
type SearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// SearchResponse represents the complete response for a single query.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// WebSearch searches the web for up-to-date information.
type WebSearch struct{}

func (w *WebSearch) Name() string {
	return "web_search"
}

func (w *WebSearch) Description() string {
	return "Search the web for real-time information using a local account-free web search provider."
}

func (w *WebSearch) Prompt() string {
	return `Use the web_search tool to search the web for current information.
Today's date is ` + time.Now().Format("January 2, 2006") + `
Add "` + time.Now().Format("January 2, 2006") + `" for news queries and ` + strconv.Itoa(time.Now().Year()+1) + ` for other queries that need current information.`
}

func (w *WebSearch) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Search query to look up"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of results to return (default: 5)",
				"default": 5
			}
		},
		"required": ["query"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (w *WebSearch) Execute(ctx context.Context, args map[string]any) (any, string, error) {
	queryRaw, ok := args["query"]
	if !ok {
		return nil, "", fmt.Errorf("query parameter is required")
	}

	query, ok := queryRaw.(string)
	if !ok || strings.TrimSpace(query) == "" {
		return nil, "", fmt.Errorf("query must be a non-empty string")
	}

	maxResults := defaultWebSearchResults
	if mr, ok := args["max_results"].(float64); ok && int(mr) > 0 {
		maxResults = int(mr)
	} else if mr, ok := args["max_results"].(int); ok && mr > 0 {
		maxResults = mr
	}

	result, err := performWebSearch(ctx, query, maxResults)
	if err != nil {
		return nil, "", err
	}

	return result, formatWebSearchResults(query, result), nil
}

func performWebSearch(ctx context.Context, query string, maxResults int) (*SearchResponse, error) {
	results, err := websearch.Search(ctx, query, maxResults)
	if err != nil {
		return nil, fmt.Errorf("web search failed: %w", err)
	}

	response := &SearchResponse{
		Results: make([]SearchResult, 0, len(results)),
	}

	for _, item := range results {
		response.Results = append(response.Results, SearchResult{
			Title:   item.Title,
			URL:     item.URL,
			Content: item.Content,
		})
	}

	return response, nil
}

func formatWebSearchResults(query string, response *SearchResponse) string {
	if response == nil || len(response.Results) == 0 {
		return fmt.Sprintf("No web search results found for %q.", query)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Web search results for %q:\n", query)
	for i, result := range response.Results {
		fmt.Fprintf(&b, "%d. %s\n", i+1, result.Title)
		fmt.Fprintf(&b, "   URL: %s\n", result.URL)
		if snippet := strings.TrimSpace(result.Content); snippet != "" {
			fmt.Fprintf(&b, "   Snippet: %s\n", truncateString(snippet, 400))
		}
	}

	return strings.TrimSpace(b.String())
}

func truncateString(input string, limit int) string {
	if limit <= 0 || len(input) <= limit {
		return input
	}
	return input[:limit]
}
