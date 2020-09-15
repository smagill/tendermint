package main

import (
	"crypto/sha256"
	"sort"
	"sync"

	abci "github.com/tendermint/tendermint/abci/types"
)

// State is the application state.
type State struct {
	sync.RWMutex
	Height   int64
	Values   map[string]string
	Hash     []byte
	Requests struct {
		InitChain abci.RequestInitChain
		CheckTx   []abci.RequestCheckTx
		DeliverTx []abci.RequestDeliverTx
	}
}

// NewState creates a new state.
func NewState() *State {
	state := &State{
		Values: make(map[string]string, 1024),
	}
	state.Hash = state.hashValues()
	return state
}

// Get fetches a value. A missing value is returned as an empty string.
func (s *State) Get(key string) string {
	s.RLock()
	defer s.RUnlock()
	return s.Values[key]
}

// Set sets a value. Setting an empty value is equivalent to deleting it.
func (s *State) Set(key, value string) {
	s.Lock()
	defer s.Unlock()
	if value == "" {
		delete(s.Values, key)
	} else {
		s.Values[key] = value
	}
}

// Commit commits the current state, possibly to disk.
func (s *State) Commit() (int64, []byte, error) {
	s.Lock()
	defer s.Unlock()
	s.Hash = s.hashValues()
	switch {
	case s.Height > 0:
		s.Height++
	case s.Requests.InitChain.InitialHeight > 0:
		s.Height = s.Requests.InitChain.InitialHeight
	default:
		s.Height = 1
	}
	return s.Height, s.Hash, nil
}

// hashValues hashes the current value set.
func (s *State) hashValues() []byte {
	keys := make([]string, 0, len(s.Values))
	for key := range s.Values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hasher := sha256.New()
	for _, key := range keys {
		_, _ = hasher.Write([]byte(key))
		_, _ = hasher.Write([]byte{0})
		_, _ = hasher.Write([]byte(s.Values[key]))
		_, _ = hasher.Write([]byte{0})
	}
	return hasher.Sum(nil)
}
