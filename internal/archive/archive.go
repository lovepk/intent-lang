package archive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Commit struct {
	ID      string    `json:"id"`
	Parent  string    `json:"parent,omitempty"`
	Message string    `json:"message"`
	UserMsg string    `json:"user_msg"`
	Archive string    `json:"archive"`
	Replied string    `json:"replied,omitempty"`
	Time    time.Time `json:"time"`
}

type Store struct {
	dir string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

func (s *Store) Dir() string { return s.dir }

func (s *Store) commitPath(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) HeadID() (string, error) {
	data, err := os.ReadFile(filepath.Join(s.dir, "HEAD"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Store) setHead(id string) error {
	return os.WriteFile(filepath.Join(s.dir, "HEAD"), []byte(id), 0o644)
}

// SetHead moves HEAD to an existing commit id (used for rollback). It does not
// delete later commits; they remain reachable by id.
func (s *Store) SetHead(id string) error {
	if _, err := s.Get(id); err != nil {
		return fmt.Errorf("no such commit %s", id)
	}
	return s.setHead(id)
}

func (s *Store) Latest() (*Commit, error) {
	head, err := s.HeadID()
	if err != nil || head == "" {
		return nil, nil
	}
	return s.Get(head)
}

func (s *Store) Get(id string) (*Commit, error) {
	data, err := os.ReadFile(s.commitPath(id))
	if err != nil {
		return nil, err
	}
	var c Commit
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Append stores a new commit. before/after are canonical archive texts (META
// ignored for diff purposes); meta provides the META section lines the Agent
// controls. The stored Archive is after re-serialized with meta attached.
func (s *Store) Append(msg, userMsg, before, after, replied string, meta []string) (*Commit, error) {
	head, err := s.HeadID()
	if err != nil {
		return nil, err
	}
	id := fmt.Sprintf("c-%d", time.Now().UnixNano())
	stored := attachMeta(after, meta)
	c := Commit{
		ID:      id,
		Parent:  head,
		Message: msg,
		UserMsg: userMsg,
		Archive: stored,
		Replied: replied,
		Time:    time.Now().UTC(),
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(s.commitPath(id), data, 0o644); err != nil {
		return nil, err
	}
	if err := s.setHead(id); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *Store) Log() ([]*Commit, error) {
	head, err := s.HeadID()
	if err != nil {
		return nil, err
	}
	var out []*Commit
	cur := head
	for cur != "" {
		c, err := s.Get(cur)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
		cur = c.Parent
	}
	return out, nil
}
