package bridge

import (
	"io"
	"log"
	"net/http"
)

// proxyToApihub 将请求代理到 apiHub，保留 query 与 body
func (s *Server) proxyToApihub(w http.ResponseWriter, r *http.Request, path string) {
	url := s.cfg.ApihubURL + path
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	var body io.Reader
	if r.Body != nil {
		body = r.Body
	}
	req, err := http.NewRequest(r.Method, url, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	for k, v := range r.Header {
		if len(v) > 0 {
			req.Header.Set(k, v[0])
		}
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("bridge: proxy %s: %v", path, err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	for k, v := range resp.Header {
		if len(v) > 0 {
			w.Header().Set(k, v[0])
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
