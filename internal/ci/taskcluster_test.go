package ci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskclusterClientTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/queue/v1/task/task_1234" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"taskQueueId":"proj/macosx1500-aarch64","payload":{"command":["echo","ok"],"env":{"A":"1"}}}`))
	}))
	defer server.Close()

	task, err := (TaskclusterClient{RootURL: server.URL}).Task(context.Background(), "task_1234")
	if err != nil {
		t.Fatal(err)
	}
	if task.TaskQueueID != "proj/macosx1500-aarch64" || task.Payload.Env["A"] != "1" {
		t.Fatalf("task = %+v", task)
	}
}
