package httpcell

import (
	"bytes"
	"context"
	"encoding/base64"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"

	"golang.org/x/crypto/sha3"
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

func (s *Server) Serve(ctx context.Context, addr string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		if err := l.Close(); err != nil {
			log.Println(err)
		}
	}()
	return http.Serve(l, s)
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
		s.newCell(p)
		w.WriteHeader(http.StatusOK)

	case http.MethodGet:
		log.Println("GET", p)
		w.WriteHeader(http.StatusOK)
		_, err := w.Write(c)
		if err != nil {
			log.Println(err)
		}

	case http.MethodPut:
		log.Println("PUT", p)
		proposed, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		believedHashb64 := r.Header.Get(currentHeader)
		believedHash, err := base64.URLEncoding.DecodeString(believedHashb64)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		s.mu.Lock()
		data := s.cs[p]
		actualHash := sha3.Sum256(data)
		if bytes.Compare(actualHash[:], believedHash) == 0 {
			s.cs[p] = proposed
			data = proposed
		}
		s.mu.Unlock()

		w.WriteHeader(http.StatusOK)
		_, err = w.Write(data)
		if err != nil {
			log.Println(err)
		}

	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (s *Server) newCell(p string) {
	s.mu.Lock()
	s.cs[p] = []byte{}
	s.mu.Unlock()
}
