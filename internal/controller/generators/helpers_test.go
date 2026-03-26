/*
Copyright 2025.

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

package generators

import (
	"testing"
)

func TestShortHash(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		h1 := shortHash("test-input")
		h2 := shortHash("test-input")
		if h1 != h2 {
			t.Errorf("shortHash is not deterministic: %q != %q", h1, h2)
		}
	})

	t.Run("length is 10", func(t *testing.T) {
		h := shortHash("anything")
		if len(h) != 10 {
			t.Errorf("shortHash length = %d, want 10", len(h))
		}
	})

	t.Run("different inputs produce different hashes", func(t *testing.T) {
		h1 := shortHash("input-a")
		h2 := shortHash("input-b")
		if h1 == h2 {
			t.Errorf("shortHash produced same hash for different inputs: %q", h1)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		h := shortHash("")
		if len(h) != 10 {
			t.Errorf("shortHash of empty string length = %d, want 10", len(h))
		}
	})
}

func TestBoolPtr(t *testing.T) {
	trueVal := boolPtr(true)
	if trueVal == nil || *trueVal != true {
		t.Error("boolPtr(true) should return pointer to true")
	}

	falseVal := boolPtr(false)
	if falseVal == nil || *falseVal != false {
		t.Error("boolPtr(false) should return pointer to false")
	}

	// Ensure they are distinct pointers
	if trueVal == falseVal {
		t.Error("boolPtr should return distinct pointers")
	}
}

func TestInt64Ptr(t *testing.T) {
	val := int64Ptr(42)
	if val == nil || *val != 42 {
		t.Error("int64Ptr(42) should return pointer to 42")
	}

	zero := int64Ptr(0)
	if zero == nil || *zero != 0 {
		t.Error("int64Ptr(0) should return pointer to 0")
	}

	neg := int64Ptr(-1)
	if neg == nil || *neg != -1 {
		t.Error("int64Ptr(-1) should return pointer to -1")
	}
}
