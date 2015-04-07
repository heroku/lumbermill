/*
Copyright 2013 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Changelog:

 - 2014-07-02 (apg): Modified to support storing a Destination instead
   of string key

*/

package main

import (
	"hash/fnv"
	"sort"
	"strconv"
)

type hashFn func(data []byte) uint32

type hashRing struct {
	hash     hashFn
	replicas int
	keys     []int // Sorted
	hashMap  map[int]*destination
}

func newHashRing(replicas int, fn hashFn) *hashRing {
	m := &hashRing{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]*destination),
	}
	if m.hash == nil {
		// Default to fnv1a since it provides a better distribution
		m.hash = func(data []byte) uint32 {
			a := fnv.New32a()
			a.Write(data)
			return a.Sum32()
		}
	}

	return m
}

// Returns true if there are no items available.
func (m *hashRing) IsEmpty() bool {
	return len(m.keys) == 0
}

// Adds some keys to the hash.
func (m *hashRing) Add(destinations ...*destination) {
	for _, destination := range destinations {
		for i := 0; i < m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + destination.Name)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = destination
		}
		sort.Ints(m.keys)
	}
}

// Gets the closest item in the hash to the provided key.
func (m *hashRing) Get(key string) *destination {
	if m.IsEmpty() {
		return nil
	}

	hash := int(m.hash([]byte(key)))

	// Binary search for appropriate replica.
	idx := sort.Search(len(m.keys), func(i int) bool { return m.keys[i] >= hash })

	// Means we have cycled back to the first replica.
	if idx == len(m.keys) {
		idx = 0
	}

	return m.hashMap[m.keys[idx]]
}
