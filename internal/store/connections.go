package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Connection struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      string    `json:"port"`
	Username  string    `json:"user"`
	Password  string    `json:"password,omitempty"`
	Database  string    `json:"database,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type Connections struct {
	path string
	mu   sync.Mutex
}

func NewConnections(path string) *Connections {
	return &Connections{path: path}
}

func (s *Connections) List() ([]Connection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Connections) Save(c Connection) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load()
	if err != nil {
		return err
	}

	c.ID = generateID()
	c.CreatedAt = time.Now()
	list = append(list, c)

	return s.persist(list)
}

func (s *Connections) load() ([]Connection, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Connection{}, nil
	}
	if err != nil {
		return nil, err
	}

	var list []Connection
	return list, json.Unmarshal(data, &list)
}

func (s *Connections) persist(list []Connection) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
