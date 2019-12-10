package httpstore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	"github.com/multiformats/go-multihash"
	"golang.org/x/crypto/sha3"
)

var enc = base64.URLEncoding

type Server struct {
	maxBlobSize int
	l           net.Listener
	m           sync.Map
}

func NewServer(laddr string, maxBlobSize int) (*Server, error) {
	l, err := net.Listen("tcp", laddr)
	if err != nil {
		return nil, err
	}
	s := &Server{
		l:           l,
		maxBlobSize: maxBlobSize,
	}
	go func() {
		if err := http.Serve(l, s); err != nil {
			log.Println(err)
		}
	}()
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.post(w, r)
	case http.MethodGet:
		if r.URL.Path == "/.maxBlobSize" {
			fmt.Fprint(w, s.maxBlobSize)
			return
		}
		s.get(w, r)
	case http.MethodHead:
		s.head(w, r)
	case http.MethodDelete:
		s.delete(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (s *Server) LocalAddr() string {
	return s.l.Addr().String()
}

func (s *Server) Close() error {
	return s.l.Close()
}

func (s *Server) GetURL() string {
	return "http://" + s.LocalAddr()
}

func (s *Server) get(w http.ResponseWriter, r *http.Request) {
	mh, err := getID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	v, ok := s.m.Load(string(mh))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(v.([]byte))
}

func (s *Server) post(w http.ResponseWriter, r *http.Request) {
	buf := make([]byte, s.maxBlobSize)

	total := 0
	for total < len(buf) {
		n, err := r.Body.Read(buf[total:])
		total += n
		if err == io.EOF {
			break
		} else if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}
	data := buf[:total]

	h := sha3.Sum256(data)
	mh, err := multihash.Encode(h[:], multihash.SHA3_256)
	if err != nil {
		panic(err)
	}

	s.m.Store(string(mh), data)

	resp := make([]byte, enc.EncodedLen(len(mh)))
	enc.Encode(resp, mh)

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

func (s *Server) head(w http.ResponseWriter, r *http.Request) {
	mh, err := getID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, exists := s.m.Load(string(mh))
	if !exists {
		w.WriteHeader(http.StatusNotFound)
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) delete(w http.ResponseWriter, r *http.Request) {
	mh, err := getID(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	_, exists := s.m.Load(string(mh))
	if !exists {
		w.WriteHeader(http.StatusNotFound)
	}
	s.m.Delete(string(mh))
	w.WriteHeader(http.StatusOK)
}

func getID(r *http.Request) ([]byte, error) {
	p := r.URL.Path
	if len(p) < 2 {
		return nil, errors.New("bad path")
	}
	p = p[1:]
	return enc.DecodeString(p)
}
