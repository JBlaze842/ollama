//go:build windows || darwin

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type WebFetch struct{}

type FetchRequest struct {
	URL string `json:"url"`
}

type FetchResponse struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Links   []string `json:"links"`
}

func (w *WebFetch) Name() string {
	return "web_fetch"
}

func (w *WebFetch) Description() string {
	return "Crawl and extract text content from web pages"
}

func (g *WebFetch) Schema() map[string]any {
	schemaBytes := []byte(`{
		"type": "object",
		"properties": {
			"url": {
				"type": "string",
				"description": "URL to crawl and extract content from"
            }
		},
		"required": ["url"]
	}`)
	var schema map[string]any
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		return nil
	}
	return schema
}

func (w *WebFetch) Prompt() string {
	return ""
}

func (w *WebFetch) Execute(ctx context.Context, args map[string]any) (any, string, error) {
	urlRaw, ok := args["url"]
	if !ok {
		return nil, "", fmt.Errorf("url parameter is required")
	}
	urlStr, ok := urlRaw.(string)
	if !ok || strings.TrimSpace(urlStr) == "" {
		return nil, "", fmt.Errorf("url must be a non-empty string")
	}

	result, err := performWebFetch(ctx, urlStr)
	if err != nil {
		return nil, "", err
	}

	return result, "", nil
}

func performWebFetch(ctx context.Context, targetURL string) (*FetchResponse, error) {
	parsedURL, err := url.Parse(strings.TrimSpace(targetURL))
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("url must use http or https")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create fetch request: %w", err)
	}

	req.Header.Set("User-Agent", webFetchUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute fetch request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch API error (status %d)", resp.StatusCode)
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.HasPrefix(contentType, "text/plain") {
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxWebFetchBodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		content := cleanFetchedText(string(body))
		return &FetchResponse{
			Title:   parsedURL.String(),
			Content: content,
		}, nil
	}

	doc, err := html.Parse(io.LimitReader(resp.Body, maxWebFetchBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to parse response html: %w", err)
	}

	title := cleanFetchedText(nodeText(findFirst(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "title"
	})))
	if title == "" {
		title = parsedURL.String()
	}

	content := cleanFetchedText(extractPageText(doc))
	links := extractLinks(doc, parsedURL)

	return &FetchResponse{
		Title:   title,
		Content: content,
		Links:   links,
	}, nil
}

const (
	webFetchUserAgent    = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
	maxWebFetchBodyBytes = 2 << 20
)

func extractPageText(root *html.Node) string {
	var b strings.Builder

	var walk func(*html.Node, bool)
	walk = func(n *html.Node, skip bool) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "noscript", "svg":
				skip = true
			case "p", "div", "section", "article", "main", "li", "br", "h1", "h2", "h3", "h4", "h5", "h6":
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
			}
		}

		if !skip && n.Type == html.TextNode {
			text := cleanFetchedText(stdhtml.UnescapeString(n.Data))
			if text != "" {
				if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n") {
					b.WriteByte(' ')
				}
				b.WriteString(text)
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, skip)
		}
	}

	walk(root, false)
	return strings.TrimSpace(b.String())
}

func extractLinks(root *html.Node, baseURL *url.URL) []string {
	seen := make(map[string]struct{})
	links := make([]string, 0, 16)

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil {
			return
		}

		if n.Type == html.ElementNode && n.Data == "a" {
			href := strings.TrimSpace(getAttr(n, "href"))
			if href != "" {
				if ref, err := url.Parse(href); err == nil {
					resolved := baseURL.ResolveReference(ref).String()
					if strings.HasPrefix(resolved, "http://") || strings.HasPrefix(resolved, "https://") {
						if _, ok := seen[resolved]; !ok {
							seen[resolved] = struct{}{}
							links = append(links, resolved)
						}
					}
				}
			}
		}

		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(root)
	return links
}

func findFirst(n *html.Node, match func(*html.Node) bool) *html.Node {
	if n == nil {
		return nil
	}

	if match(n) {
		return n
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := findFirst(child, match); found != nil {
			return found
		}
	}

	return nil
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}

	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
			b.WriteByte(' ')
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}

	walk(n)
	return b.String()
}

func cleanFetchedText(s string) string {
	lines := strings.Split(s, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(strings.TrimSpace(line)), " ")
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return strings.Join(cleaned, "\n")
}
