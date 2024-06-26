// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/lithammer/dedent"
)

func TestServerHandler(t *testing.T) {
	tests := []struct {
		name       string
		cql        string
		data       string
		bodyJSON   string
		wantOutput string
	}{
		{
			name: "Simple CQL with no data",
			cql: dedent.Dedent(`
			library Explore version '1.2.3'
			define result: 1+1`),
			wantOutput: `[
				{
					"libName": "Explore",
					"libVersion": "1.2.3",
					"expressionDefinitions": {
						"result": {
								"@type": "System.Integer",
								"value": 2
							}
					}
				}
			]`,
		},
		{
			name: "CQL with patient data access",
			cql: dedent.Dedent(`
			library Explore version '1.2.3'
			using FHIR version '4.0.1'
			include FHIRHelpers version '4.0.1' called FHIRHelpers
			context Patient
			define result: Patient.id.value`),
			data: `{
				"resourceType": "Bundle",
				"type": "transaction",
				"entry": [
					{
						"fullUrl": "fullUrl",
						"resource": {
							"resourceType": "Patient",
							"id": "1"}
					}
				]
			}`,
			wantOutput: `[
				{
					"libName": "Explore",
					"libVersion": "1.2.3",
					"expressionDefinitions": {
						"result": {
								"@type": "System.String",
								"value": "1"
							}
					}
				}
			]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h, err := serverHandler()
			if err != nil {
				t.Fatalf("serverHandler() returned an unexpected error: %v", err)
			}
			server := httptest.NewServer(h)
			defer server.Close()

			bodyJSON := fmt.Sprintf(`{"cql": %q, "data": %q}`, tc.cql, tc.data)
			resp, err := http.Post(server.URL+"/eval_cql", "application/json", strings.NewReader(bodyJSON))
			if err != nil {
				t.Fatalf("http.Post(%v) with body %v returned an unexpected error: %v", server.URL, bodyJSON, err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("io.ReadAll(%v) returned an unexpected error: %v", resp.Body, err)
			}
			got := string(body)
			if !cmp.Equal(normalizeJSON(t, got), normalizeJSON(t, tc.wantOutput)) {
				t.Errorf("POST to /eval_cql to CQL server with body %v returned %v, want %v", tc.bodyJSON, got, tc.wantOutput)
			}
		})
	}
}

// TODO: b/301659936 - Add tests that build and run the largetest examples in the CQL repo.

func normalizeJSON(t *testing.T, s string) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("json.Unmarshal() returned an unexpected error: %v", err)
	}
	outBytes, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal() returned an unexpected error: %v", err)
	}
	return outBytes
}
