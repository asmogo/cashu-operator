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
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

func TestMnemonicSecretName(t *testing.T) {
	if got, want := MnemonicSecretName("mint-a"), "mint-a-mnemonic"; got != want {
		t.Fatalf("MnemonicSecretName() = %q, want %q", got, want)
	}
}

func TestGenerateMnemonicSecret_GeneratesValidMnemonic(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("mnemonic-generated")
	mint.UID = types.UID("mnemonic-generated-uid")

	secret, err := GenerateMnemonicSecret(mint, scheme, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret.Name != "mnemonic-generated-mnemonic" {
		t.Fatalf("name = %q, want mnemonic-generated-mnemonic", secret.Name)
	}
	if secret.Namespace != mint.Namespace {
		t.Fatalf("namespace = %q, want %q", secret.Namespace, mint.Namespace)
	}
	assertLabelsContain(t, secret.Labels, "app.kubernetes.io/instance", mint.Name)
	assertLabelsContain(t, secret.Labels, "app.kubernetes.io/component", "mnemonic")
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].Name != mint.Name {
		t.Fatalf("unexpected owner refs: %+v", secret.OwnerReferences)
	}
	if secret.OwnerReferences[0].Controller == nil || !*secret.OwnerReferences[0].Controller {
		t.Fatalf("expected controller owner ref, got %+v", secret.OwnerReferences[0])
	}

	mnemonic := secret.StringData[MnemonicSecretKey]
	if mnemonic == "" {
		t.Fatal("expected generated mnemonic in StringData")
	}
	if err := validateBIP39Mnemonic(mnemonic); err != nil {
		t.Fatalf("generated mnemonic is not valid BIP39: %v", err)
	}
}

func TestGenerateMnemonicSecret_UsesExistingMnemonic(t *testing.T) {
	scheme := testScheme(t)
	mint := baseMint("mnemonic-existing")
	existing := "abandon ability able about above absent absorb abstract absurd abuse access accident account accuse achieve acid acoustic acquire across act action actor actress actual"

	secret, err := GenerateMnemonicSecret(mint, scheme, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := secret.StringData[MnemonicSecretKey]; got != existing {
		t.Fatalf("mnemonic = %q, want existing mnemonic", got)
	}
}

func TestGenerateBIP39Mnemonic(t *testing.T) {
	first, err := generateBIP39Mnemonic()
	if err != nil {
		t.Fatalf("unexpected error generating first mnemonic: %v", err)
	}
	second, err := generateBIP39Mnemonic()
	if err != nil {
		t.Fatalf("unexpected error generating second mnemonic: %v", err)
	}
	if first == second {
		t.Fatal("expected independently generated mnemonics to differ")
	}
	if err := validateBIP39Mnemonic(first); err != nil {
		t.Fatalf("first mnemonic is not valid BIP39: %v", err)
	}
	if err := validateBIP39Mnemonic(second); err != nil {
		t.Fatalf("second mnemonic is not valid BIP39: %v", err)
	}
}

func validateBIP39Mnemonic(mnemonic string) error {
	words := strings.Fields(mnemonic)
	if len(words) != 24 {
		return fmt.Errorf("word count = %d, want 24", len(words))
	}

	indexes := make(map[string]int, len(bip39WordList))
	for i, word := range bip39WordList {
		indexes[word] = i
	}

	var bitString strings.Builder
	for _, word := range words {
		index, ok := indexes[word]
		if !ok {
			return fmt.Errorf("word %q is not in BIP39 word list", word)
		}
		bitString.WriteString(fmt.Sprintf("%011b", index))
	}

	bits := bitString.String()
	if len(bits) != 264 {
		return fmt.Errorf("combined bit length = %d, want 264", len(bits))
	}

	entropy := make([]byte, 32)
	for i := range entropy {
		value, err := strconv.ParseUint(bits[i*8:(i+1)*8], 2, 8)
		if err != nil {
			return fmt.Errorf("failed to parse entropy byte %d: %w", i, err)
		}
		entropy[i] = byte(value)
	}

	hash := sha256.Sum256(entropy)
	expectedChecksum := fmt.Sprintf("%08b", hash[0])
	if bits[256:] != expectedChecksum {
		return fmt.Errorf("checksum mismatch: got %s want %s", bits[256:], expectedChecksum)
	}

	return nil
}
