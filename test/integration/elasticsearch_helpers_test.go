//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func elasticURL(t *testing.T) string {
	t.Helper()
	return requireEnv(t, "ELASTICSEARCH_URL")
}

func newElasticIndex(prefix string) string {
	return uniqueName(prefix)
}

func cleanupElasticIndex(t *testing.T, baseURL, index string) {
	t.Helper()
	t.Cleanup(func() {
		request, err := http.NewRequest(http.MethodDelete, baseURL+"/"+url.PathEscape(index), nil)
		if err != nil {
			return
		}
		response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
		if err == nil {
			_ = response.Body.Close()
		}
	})
}

func esRequest(t *testing.T, method, requestURL string) (int, []byte) {
	t.Helper()

	request, err := http.NewRequest(method, requestURL, nil)
	if err != nil {
		t.Fatalf("build elasticsearch request: %v", err)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("elasticsearch request %s %s: %v", method, requestURL, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		t.Fatalf("read elasticsearch response: %v", err)
	}
	return response.StatusCode, body
}

func elasticCount(t *testing.T, baseURL, index string) (int, error) {
	t.Helper()

	status, body := esRequest(t, http.MethodPost, baseURL+"/"+url.PathEscape(index)+"/_refresh")
	if status == http.StatusNotFound {
		return 0, nil
	}
	if status >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("refresh index %s: status %d: %s", index, status, body)
	}

	status, body = esRequest(t, http.MethodGet, baseURL+"/"+url.PathEscape(index)+"/_count")
	if status == http.StatusNotFound {
		return 0, nil
	}
	if status >= http.StatusMultipleChoices {
		return 0, fmt.Errorf("count index %s: status %d: %s", index, status, body)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("parse count response: %w", err)
	}
	return result.Count, nil
}

func waitForElasticCount(t *testing.T, baseURL, index string, want int) {
	t.Helper()
	waitFor(t, 30*time.Second, 300*time.Millisecond, fmt.Sprintf("elasticsearch index %s reaching %d documents", index, want), func() (bool, string) {
		got, err := elasticCount(t, baseURL, index)
		if err != nil {
			return false, err.Error()
		}
		return got == want, fmt.Sprintf("documents = %d", got)
	})
}

func elasticMessages(t *testing.T, baseURL, index string) []string {
	t.Helper()

	status, body := esRequest(t, http.MethodPost, baseURL+"/"+url.PathEscape(index)+"/_refresh")
	if status >= http.StatusMultipleChoices {
		t.Fatalf("refresh index %s: status %d: %s", index, status, body)
	}

	status, body = esRequest(t, http.MethodGet, baseURL+"/"+url.PathEscape(index)+"/_search?size=1000")
	if status >= http.StatusMultipleChoices {
		t.Fatalf("search index %s: status %d: %s", index, status, body)
	}

	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					Message string `json:"message"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("parse search response: %v", err)
	}

	messages := make([]string, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		messages = append(messages, hit.Source.Message)
	}
	return messages
}
