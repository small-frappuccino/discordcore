package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

func main() {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/manage/settings/guilds?tab=access", nil)
	http.Redirect(rec, req, "http://127.0.0.1:8080/", http.StatusFound)
	fmt.Printf("Location: %q\n", rec.Header().Get("Location"))
}
