package websearch

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestExtractResultsParsesDuckDuckGoMarkup(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`
<!doctype html>
<html>
  <body>
    <div class="result">
      <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Falpha">Alpha Result</a>
      <div class="result__snippet">Alpha snippet text</div>
    </div>
    <article class="result result--web">
      <a href="https://example.com/beta">Beta Result</a>
      <div class="result__body">Beta snippet text</div>
    </article>
  </body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	results := extractResults(root, 5)
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if results[0].Title != "Alpha Result" {
		t.Fatalf("results[0].Title = %q, want %q", results[0].Title, "Alpha Result")
	}
	if results[0].URL != "https://example.com/alpha" {
		t.Fatalf("results[0].URL = %q, want %q", results[0].URL, "https://example.com/alpha")
	}
	if results[0].Content != "Alpha snippet text" {
		t.Fatalf("results[0].Content = %q, want %q", results[0].Content, "Alpha snippet text")
	}

	if results[1].Title != "Beta Result" {
		t.Fatalf("results[1].Title = %q, want %q", results[1].Title, "Beta Result")
	}
	if results[1].URL != "https://example.com/beta" {
		t.Fatalf("results[1].URL = %q, want %q", results[1].URL, "https://example.com/beta")
	}
	if results[1].Content != "Beta snippet text" {
		t.Fatalf("results[1].Content = %q, want %q", results[1].Content, "Beta snippet text")
	}
}

func TestExtractResultsHonorsMaxResults(t *testing.T) {
	root, err := html.Parse(strings.NewReader(`
<html>
  <body>
    <div class="result"><a href="https://example.com/one">One</a></div>
    <div class="result"><a href="https://example.com/two">Two</a></div>
  </body>
</html>`))
	if err != nil {
		t.Fatalf("parse html: %v", err)
	}

	results := extractResults(root, 1)
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].Title != "One" {
		t.Fatalf("results[0].Title = %q, want %q", results[0].Title, "One")
	}
}

func TestDecodeDuckDuckGoURL(t *testing.T) {
	got := decodeDuckDuckGoURL("//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpath")
	want := "https://example.com/path"
	if got != want {
		t.Fatalf("decodeDuckDuckGoURL() = %q, want %q", got, want)
	}
}
