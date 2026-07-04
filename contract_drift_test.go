package tossinvest

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	tossapi "github.com/awuzag/tossinvest-go/internal/generated/tossapi"
)

type contractSpec struct {
	Paths      map[string]map[string]contractOperation `json:"paths"`
	Components struct {
		Parameters map[string]contractParameter `json:"parameters"`
	} `json:"components"`
}

type contractOperation struct {
	OperationID string              `json:"operationId"`
	Tags        []string            `json:"tags"`
	Summary     string              `json:"summary"`
	Parameters  []contractParameter `json:"parameters"`
}

type contractParameter struct {
	Ref  string `json:"$ref"`
	Name string `json:"name"`
	In   string `json:"in"`
}

func TestGeneratedCatalogMatchesOpenAPIContract(t *testing.T) {
	data, err := os.ReadFile("contracts/tossinvest/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec contractSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	expected := map[string]tossapi.OperationMetadata{}
	for path, item := range spec.Paths {
		for method, operation := range item {
			if !isHTTPMethod(method) || operation.OperationID == "" {
				continue
			}
			tag := ""
			if len(operation.Tags) > 0 {
				tag = operation.Tags[0]
			}
			expected[operation.OperationID] = tossapi.OperationMetadata{
				OperationID:   operation.OperationID,
				Tag:           tag,
				Method:        strings.ToUpper(method),
				Path:          path,
				Summary:       operation.Summary,
				AccountScoped: contractAccountScoped(spec, operation),
				LiveTrading:   contractLiveTrading(operation.OperationID),
			}
		}
	}

	generated := map[string]tossapi.OperationMetadata{}
	for _, operation := range tossapi.Operations() {
		generated[operation.OperationID] = operation
	}
	if len(generated) != len(expected) {
		t.Fatalf("operation count drift: generated=%d contract=%d", len(generated), len(expected))
	}
	for operationID, want := range expected {
		got, ok := generated[operationID]
		if !ok {
			t.Fatalf("generated catalog is missing operationId %q", operationID)
		}
		if got != want {
			t.Fatalf("operation metadata drift for %s:\nwant=%#v\ngot =%#v", operationID, want, got)
		}
	}
	for operationID := range generated {
		if _, ok := expected[operationID]; !ok {
			t.Fatalf("generated catalog has unknown operationId %q", operationID)
		}
	}
}

func contractAccountScoped(spec contractSpec, operation contractOperation) bool {
	for _, parameter := range operation.Parameters {
		if parameter.Ref != "" {
			parameter = spec.Components.Parameters[refName(parameter.Ref)]
		}
		if parameter.In == "header" && strings.EqualFold(parameter.Name, "X-Tossinvest-Account") {
			return true
		}
	}
	return false
}

func contractLiveTrading(operationID string) bool {
	switch operationID {
	case "createOrder", "modifyOrder", "cancelOrder":
		return true
	default:
		return false
	}
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "post", "put", "patch", "delete":
		return true
	default:
		return false
	}
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}
