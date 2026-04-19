package websearch

import (
	"context"
	"fmt"
	stdhtml "html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	defaultMaxResults = 5
	searchEndpoint    = "https://html.duckduckgo.com/html/"
	userAgent         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"
)

type Result struct {
	Title   string
	URL     string
	Content string
}

func Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}

	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	reqURL, err := url.Parse(searchEndpoint)
	if err != nil {
		return nil, fmt.Errorf("parse search endpoint: %w", err)
	}

	q := reqURL.Query()
	q.Set("q", query)
	q.Set("kl", "us-en")
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search returned status %d", resp.StatusCode)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse search response html: %w", err)
	}

	results := extractResults(doc, maxResults)
	return results, nil
}

func extractResults(root *html.Node, maxResults int) []Result {
	var results []Result

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n == nil || len(results) >= maxResults {
			return
		}

		if n.Type == html.ElementNode && isResultContainer(n) {
			if result, ok := parseResult(n); ok {
				results = append(results, result)
			}
		}

		for child := n.FirstChild; child != nil && len(results) < maxResults; child = child.NextSibling {
			walk(child)
		}
	}

	walk(root)
	return results
}

func isResultContainer(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}

	if n.Data != "div" && n.Data != "article" {
		return false
	}

	class := getAttr(n, "class")
	return hasClassToken(class, "result")
}

func parseResult(n *html.Node) (Result, bool) {
	link := findFirst(n, func(node *html.Node) bool {
		return node.Type == html.ElementNode &&
			node.Data == "a" &&
			(hasClassToken(getAttr(node, "class"), "result__a") ||
				hasClassToken(getAttr(node, "class"), "result__url"))
	})

	if link == nil {
		link = findFirst(n, func(node *html.Node) bool {
			return node.Type == html.ElementNode &&
				node.Data == "a" &&
				getAttr(node, "href") != ""
		})
	}

	if link == nil {
		return Result{}, false
	}

	title := cleanText(nodeText(link))
	href := strings.TrimSpace(getAttr(link, "href"))
	if title == "" || href == "" {
		return Result{}, false
	}

	resolvedURL := decodeDuckDuckGoURL(href)
	if resolvedURL == "" {
		return Result{}, false
	}

	snippetNode := findFirst(n, func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return false
		}
		class := getAttr(node, "class")
		return hasClassToken(class, "result__snippet") || hasClassToken(class, "result__body")
	})

	snippet := ""
	if snippetNode != nil {
		snippet = cleanText(nodeText(snippetNode))
	}

	return Result{
		Title:   title,
		URL:     resolvedURL,
		Content: snippet,
	}, true
}

func decodeDuckDuckGoURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	if uddg := u.Query().Get("uddg"); uddg != "" {
		decoded, err := url.QueryUnescape(uddg)
		if err == nil && decoded != "" {
			return decoded
		}
		return uddg
	}

	return raw
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

func hasClassToken(classValue, token string) bool {
	for _, part := range strings.Fields(classValue) {
		if part == token || strings.Contains(part, token) {
			return true
		}
	}
	return false
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
	return stdhtml.UnescapeString(b.String())
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}
