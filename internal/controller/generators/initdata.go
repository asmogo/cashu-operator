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
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"

	mintv1alpha1 "github.com/asmogo/cashu-operator/api/v1alpha1"
)

// generateTEEInitData builds the cc_init_data annotation value for a TEE pod VM.
//
// It renders an initdata TOML containing the Attestation Agent config (aa.toml)
// and Confidential Data Hub config (cdh.toml), then encodes it as
// base64(gzip(toml)) — exactly the format cloud-api-adaptor's initdata.Encode
// produces and the `io.katacontainers.config.hypervisor.cc_init_data`
// annotation consumes.
//
// cdh.toml's [kbc] url is the single source for the KBS endpoint: the CDH
// exports it as AA_KBC_PARAMS at startup, which the KMS "kbs" getter requires
// to fetch + unseal the sealed-secret reference.
func generateTEEInitData(tee *mintv1alpha1.TEEConfig) (string, error) {
	if tee == nil || tee.KBS == nil || tee.KBS.URL == "" {
		return "", fmt.Errorf("tee.kbs.url is required to generate initdata")
	}

	skipVerification := true
	if tee.KBS.SkipSealedSecretVerification != nil {
		skipVerification = *tee.KBS.SkipSealedSecretVerification
	}

	toml := fmt.Sprintf(`algorithm = "sha384"
version = "0.1.0"

[data]
"aa.toml" = """
[token_configs]
[token_configs.kbs]
url = "%s"

[eventlog_config]
init_pcr = 17
enable_eventlog = false
"""

"cdh.toml" = """
socket = "unix:///run/confidential-containers/cdh.sock"
skip_sealed_secret_verification = %t
[kbc]
name = "cc_kbc"
url = "%s"
"""
`, tee.KBS.URL, skipVerification, tee.KBS.URL)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(toml)); err != nil {
		return "", fmt.Errorf("gzip initdata: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("gzip close: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}
