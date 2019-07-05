package httpcell

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

type Server struct {
	mu sync.Mutex
	cs map[string][]byte
}

func NewServer() *Server {
	return &Server{
		cs: map[string][]byte{},
	}
}

func (s *Server) ListenAndServe(addr string) error {
	return http.ListenAndServe(addr, s)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	c, exists := s.cs[p]

	if r.Method != http.MethodPost && !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		s.mu.Lock()
		s.cs[p] = []byte{}
		s.mu.Unlock()
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(c)
		if err != nil {
			log.Println(err)
		}

	case http.MethodPut:
		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		casReq := CASReq{}
		if err := json.Unmarshal(reqBody, &casReq); err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		casRes := CASRes{}
		if bytes.Compare(c, casReq.Current) == 0 {
			casRes.Changed = true
			s.cs[p] = casReq.Next
		}
		casRes.Current = s.cs[p]
		s.mu.Unlock()

		resBody, _ := json.Marshal(casRes)
		w.WriteHeader(http.StatusOK)
		_, err = w.Write(resBody)
		if err != nil {
			log.Println(err)
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}
