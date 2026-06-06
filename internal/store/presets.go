package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Preset struct {
	ID        string    `json:"id,omitempty"`
	Name      string    `json:"name,omitempty"`
	SrcConn   string    `json:"src_connection"`
	SrcDB     string    `json:"src_database"`
	SrcTable  string    `json:"src_table"`
	DstConn   string    `json:"dst_connection"`
	DstDB     string    `json:"dst_database"`
	DstTable  string    `json:"dst_table,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type Presets struct {
	path string
	mu   sync.Mutex
}

func NewPresets(path string) *Presets {
	return &Presets{path: path}
}

func (s *Presets) List() ([]Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.load()
}

func (s *Presets) Save(p Preset) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load()
	if err != nil {
		return err
	}
	p.ID = generateID()
	p.CreatedAt = time.Now()
	list = append(list, p)
	return s.persist(list)
}

func (s *Presets) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load()
	if err != nil {
		return err
	}
	out := list[:0]
	for _, p := range list {
		if p.ID != id {
			out = append(out, p)
		}
	}
	return s.persist(out)
}

func (s *Presets) GetByID(id string) (*Preset, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, err := s.load()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("preset %q not found", id)
}

func (s *Presets) load() ([]Preset, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return []Preset{}, nil
	}
	if err != nil {
		return nil, err
	}
	var list []Preset
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	needsSave := false
	for i := range list {
		if list[i].ID == "" {
			list[i].ID = generateID()
			needsSave = true
		}
	}
	if needsSave {
		_ = s.persist(list)
	}
	return list, nil
}

func (s *Presets) persist(list []Preset) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}
