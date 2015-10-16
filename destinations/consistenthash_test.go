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

*/

package destinations

import (
	"hash/crc32"
	"strconv"
	"testing"
)

func TestHashing(t *testing.T) {

	// Override the hash function to return easier to reason about values. Assumes
	// the keys can be converted to an integer.
	hash := newHashRing(3, func(key []byte) uint32 {
		i, err := strconv.Atoi(string(key))
		if err != nil {
			panic(err)
		}
		return uint32(i)
	})

	two := newDestination("2", 1)
	four := newDestination("4", 1)
	six := newDestination("6", 1)
	eight := newDestination("8", 1)

	// Given the above hash function, this will give replicas with "hashes":
	// 2, 4, 6, 12, 14, 16, 22, 24, 26
	hash.Add(six, four, two)

	testCases := map[string]*destination{
		"2":  two,
		"11": two,
		"23": four,
		"27": two,
	}

	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %v", k, v)
		}
	}

	// Adds 8
	hash.Add(eight)

	// 27 should now map to 8.
	testCases["27"] = eight

	for k, v := range testCases {
		if hash.Get(k) != v {
			t.Errorf("Asking for %s, should have yielded %s", k, v.Name)
		}
	}

}

func TestConsistency(t *testing.T) {
	hash1 := newHashRing(1, crc32.ChecksumIEEE)
	hash2 := newHashRing(1, crc32.ChecksumIEEE)

	ben := newDestination("Ben", 1)
	becky := newDestination("Becky", 1)
	bill := newDestination("Bill", 1)
	bob := newDestination("Bob", 1)
	bobby := newDestination("Bobby", 1)
	bonny := newDestination("Bonny", 1)

	hash1.Add(bill, bob, bonny)
	hash2.Add(bob, bonny, bill)

	if hash1.Get("Ben") != hash2.Get("Ben") {
		t.Errorf("Fetching 'Ben' from both hashes should be the same")
	}

	hash2.Add(becky, ben, bobby)

	if hash1.Get("Ben") != hash2.Get("Ben") ||
		hash1.Get("Bob") != hash2.Get("Bob") ||
		hash1.Get("Bonny") != hash2.Get("Bonny") {
		t.Errorf("Direct matches should always return the same entry")
	}
}
