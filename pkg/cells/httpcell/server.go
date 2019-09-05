package httpcell

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"

	"golang.org/x/crypto/sha3"
)

type Server struct {
	mu        sync.Mutex
	setupWait sync.WaitGroup
	cs        map[string][]byte
	l         net.Listener
}

func NewServer() *Server {
	s := &Server{
		cs:        map[string][]byte{},
		setupWait: sync.WaitGroup{},
	}
	s.setupWait.Add(1)
	return s
}

func (s *Server) Serve(ctx context.Context, laddr string) (err error) {
	s.l, err = net.Listen("tcp", laddr)
	if err != nil {
		return err
	}
	s.setupWait.Done()

	go func() {
		<-ctx.Done()
		if err := s.l.Close(); err != nil {
			log.Println(err)
		}
	}()
	return http.Serve(s.l, s)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	if len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}

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

func (s *Server) URL() string {
	s.setupWait.Wait()
	return fmt.Sprintf("http://%s/", s.l.Addr())
}

func (s *Server) CreateCell(name string) Spec {
	s.newCell(name)
	return Spec{URL: s.URL() + name}
}

func (s *Server) newCell(p string) {
	s.mu.Lock()
	s.cs[p] = []byte{}
	s.mu.Unlock()
}
