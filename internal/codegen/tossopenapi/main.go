package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

const generatorName = "internal/codegen/tossopenapi"

type openAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Info       infoObject          `json:"info"`
	Paths      map[string]pathItem `json:"paths"`
	Components componentsObject    `json:"components"`
}

type infoObject struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type componentsObject struct {
	Schemas    map[string]schemaObject    `json:"schemas"`
	Parameters map[string]parameterObject `json:"parameters"`
}

type pathItem map[string]operationObject

type operationObject struct {
	OperationID string                    `json:"operationId"`
	Tags        []string                  `json:"tags"`
	Summary     string                    `json:"summary"`
	Description string                    `json:"description"`
	Parameters  []parameterObject         `json:"parameters"`
	RequestBody *requestBodyObject        `json:"requestBody"`
	Responses   map[string]responseObject `json:"responses"`
}

type parameterObject struct {
	Ref         string       `json:"$ref"`
	Name        string       `json:"name"`
	In          string       `json:"in"`
	Required    bool         `json:"required"`
	Description string       `json:"description"`
	Schema      schemaObject `json:"schema"`
}

type requestBodyObject struct {
	Ref      string                     `json:"$ref"`
	Required bool                       `json:"required"`
	Content  map[string]mediaTypeObject `json:"content"`
}

type responseObject struct {
	Ref     string                     `json:"$ref"`
	Content map[string]mediaTypeObject `json:"content"`
}

type mediaTypeObject struct {
	Schema schemaObject `json:"schema"`
}

type schemaObject struct {
	Ref         string                  `json:"$ref"`
	Type        any                     `json:"type"`
	Format      string                  `json:"format"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Required    []string                `json:"required"`
	Properties  map[string]schemaObject `json:"properties"`
	Items       *schemaObject           `json:"items"`
	AllOf       []schemaObject          `json:"allOf"`
	OneOf       []schemaObject          `json:"oneOf"`
	AnyOf       []schemaObject          `json:"anyOf"`
	Enum        []any                   `json:"enum"`
}

type operationSpec struct {
	OperationID    string
	GoName         string
	Tag            string
	Method         string
	Path           string
	Summary        string
	AccountScoped  bool
	LiveTrading    bool
	ExpectEnvelope bool
	Parameters     []paramSpec
	RequestBody    *bodySpec
	ResponseGoType string
}

type paramSpec struct {
	Name      string
	Source    string
	FieldName string
	GoType    string
	Required  bool
}

type bodySpec struct {
	ContentType string
	GoType      string
	Form        bool
	EmptyObject bool
}

func main() {
	specPath := flag.String("spec", "contracts/tossinvest/openapi.json", "Toss Invest OpenAPI contract path")
	outDir := flag.String("out", "internal/generated/tossapi", "generated package output directory")
	flag.Parse()

	spec, err := loadSpec(*specPath)
	if err != nil {
		fatalf("%v", err)
	}
	ops, err := buildOperations(spec)
	if err != nil {
		fatalf("%v", err)
	}
	if err := writeGenerated(*outDir, spec, ops); err != nil {
		fatalf("%v", err)
	}
}

func loadSpec(path string) (openAPISpec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return openAPISpec{}, fmt.Errorf("read spec: %w", err)
	}
	var spec openAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return openAPISpec{}, fmt.Errorf("decode spec: %w", err)
	}
	return spec, nil
}

func buildOperations(spec openAPISpec) ([]operationSpec, error) {
	paths := sortedKeys(spec.Paths)
	ops := make([]operationSpec, 0)
	for _, path := range paths {
		item := spec.Paths[path]
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			op, ok := item[method]
			if !ok {
				continue
			}
			built, err := buildOperation(spec, strings.ToUpper(method), path, op)
			if err != nil {
				return nil, err
			}
			ops = append(ops, built)
		}
	}
	sort.SliceStable(ops, func(i, j int) bool {
		if ops[i].Path == "/oauth2/token" {
			return true
		}
		if ops[j].Path == "/oauth2/token" {
			return false
		}
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		return ops[i].Method < ops[j].Method
	})
	return ops, nil
}

func buildOperation(spec openAPISpec, method string, path string, op operationObject) (operationSpec, error) {
	if strings.TrimSpace(op.OperationID) == "" {
		return operationSpec{}, fmt.Errorf("%s %s is missing operationId", method, path)
	}
	params := make([]paramSpec, 0, len(op.Parameters))
	accountScoped := false
	for _, raw := range op.Parameters {
		param, err := resolveParameter(spec, raw)
		if err != nil {
			return operationSpec{}, fmt.Errorf("%s: %w", op.OperationID, err)
		}
		fieldName := parameterFieldName(param)
		if fieldName == "AccountSeq" && param.In == "header" {
			accountScoped = true
		}
		params = append(params, paramSpec{
			Name:      param.Name,
			Source:    param.In,
			FieldName: fieldName,
			GoType:    parameterGoType(spec, param),
			Required:  param.Required,
		})
	}

	body, err := buildRequestBody(spec, op.RequestBody)
	if err != nil {
		return operationSpec{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	responseType, envelope, err := successResponseType(spec, op)
	if err != nil {
		return operationSpec{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	tag := ""
	if len(op.Tags) > 0 {
		tag = op.Tags[0]
	}
	return operationSpec{
		OperationID:    op.OperationID,
		GoName:         goName(op.OperationID),
		Tag:            tag,
		Method:         method,
		Path:           path,
		Summary:        cleanText(op.Summary),
		AccountScoped:  accountScoped,
		LiveTrading:    isLiveTradingOperation(op.OperationID),
		ExpectEnvelope: envelope,
		Parameters:     params,
		RequestBody:    body,
		ResponseGoType: responseType,
	}, nil
}

func buildRequestBody(spec openAPISpec, body *requestBodyObject) (*bodySpec, error) {
	if body == nil {
		return nil, nil
	}
	if body.Ref != "" {
		return nil, fmt.Errorf("requestBody $ref is not supported: %s", body.Ref)
	}
	if media, ok := body.Content["application/json"]; ok {
		if media.Schema.Ref != "" {
			return &bodySpec{ContentType: "application/json", GoType: goName(refName(media.Schema.Ref))}, nil
		}
		if len(media.Schema.Properties) == 0 && hasType(media.Schema, "object") {
			return &bodySpec{ContentType: "application/json", EmptyObject: true}, nil
		}
		return nil, fmt.Errorf("unsupported inline JSON request body")
	}
	if media, ok := body.Content["application/x-www-form-urlencoded"]; ok {
		if media.Schema.Ref == "" {
			return nil, fmt.Errorf("form request body must use a component schema")
		}
		return &bodySpec{ContentType: "application/x-www-form-urlencoded", GoType: goName(refName(media.Schema.Ref)), Form: true}, nil
	}
	return nil, fmt.Errorf("unsupported request body content")
}

func successResponseType(spec openAPISpec, op operationObject) (string, bool, error) {
	resp, ok := op.Responses["200"]
	if !ok {
		return "", false, fmt.Errorf("missing 200 response")
	}
	media, ok := resp.Content["application/json"]
	if !ok {
		return "", false, fmt.Errorf("missing application/json 200 response")
	}
	schema := media.Schema
	if schema.Ref != "" {
		return goName(refName(schema.Ref)), false, nil
	}
	for _, part := range schema.AllOf {
		if result, ok := part.Properties["result"]; ok {
			return goType(spec, result, true), true, nil
		}
	}
	return "", false, fmt.Errorf("unsupported 200 response schema")
}

func writeGenerated(outDir string, spec openAPISpec, ops []operationSpec) error {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := removeGeneratedFiles(outDir); err != nil {
		return err
	}
	files := map[string]string{
		"catalog_gen.go": generatedCatalog(spec, ops),
		"types_gen.go":   generatedTypes(spec),
		"client_gen.go":  generatedClient(ops),
	}
	for name, source := range files {
		if err := writeGo(filepath.Join(outDir, name), source); err != nil {
			return err
		}
	}
	return nil
}

func generatedCatalog(spec openAPISpec, ops []operationSpec) string {
	var buf bytes.Buffer
	writeGeneratedHeader(&buf)
	buf.WriteString("package tossapi\n\n")
	buf.WriteString("type OperationMetadata struct {\n")
	buf.WriteString("\tOperationID string `json:\"operationId\"`\n")
	buf.WriteString("\tTag string `json:\"tag\"`\n")
	buf.WriteString("\tMethod string `json:\"method\"`\n")
	buf.WriteString("\tPath string `json:\"path\"`\n")
	buf.WriteString("\tSummary string `json:\"summary\"`\n")
	buf.WriteString("\tAccountScoped bool `json:\"accountScoped\"`\n")
	buf.WriteString("\tLiveTrading bool `json:\"liveTrading\"`\n")
	buf.WriteString("}\n\n")
	fmt.Fprintf(&buf, "const ContractOpenAPIVersion = %q\n", spec.OpenAPI)
	fmt.Fprintf(&buf, "const ContractAPIVersion = %q\n\n", spec.Info.Version)
	buf.WriteString("const (\n")
	for _, op := range ops {
		fmt.Fprintf(&buf, "\tOperation%s = %q\n", op.GoName, op.OperationID)
		fmt.Fprintf(&buf, "\tEndpoint%s = %q\n", op.GoName, op.Path)
	}
	buf.WriteString(")\n\n")
	buf.WriteString("var operationOrder = []string{\n")
	for _, op := range ops {
		fmt.Fprintf(&buf, "\tOperation%s,\n", op.GoName)
	}
	buf.WriteString("}\n\n")
	buf.WriteString("var operationMetadata = map[string]OperationMetadata{\n")
	for _, op := range ops {
		fmt.Fprintf(&buf, "\tOperation%s: {OperationID: Operation%s, Tag: %q, Method: %q, Path: Endpoint%s, Summary: %q, AccountScoped: %t, LiveTrading: %t},\n",
			op.GoName, op.GoName, op.Tag, op.Method, op.GoName, op.Summary, op.AccountScoped, op.LiveTrading)
	}
	buf.WriteString("}\n\n")
	buf.WriteString("func Operations() []OperationMetadata {\n")
	buf.WriteString("\tout := make([]OperationMetadata, 0, len(operationOrder))\n")
	buf.WriteString("\tfor _, operationID := range operationOrder {\n")
	buf.WriteString("\t\tif metadata, ok := operationMetadata[operationID]; ok {\n")
	buf.WriteString("\t\t\tout = append(out, metadata)\n")
	buf.WriteString("\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")
	buf.WriteString("func LookupOperation(operationID string) (OperationMetadata, bool) {\n")
	buf.WriteString("\tmetadata, ok := operationMetadata[operationID]\n")
	buf.WriteString("\treturn metadata, ok\n")
	buf.WriteString("}\n")
	return buf.String()
}

func generatedTypes(spec openAPISpec) string {
	var buf bytes.Buffer
	writeGeneratedHeader(&buf)
	buf.WriteString("package tossapi\n\n")
	names := sortedKeys(spec.Components.Schemas)
	for _, schemaName := range names {
		writeSchemaType(&buf, spec, schemaName, spec.Components.Schemas[schemaName])
	}
	return buf.String()
}

func writeSchemaType(buf *bytes.Buffer, spec openAPISpec, schemaName string, schema schemaObject) {
	typeName := goName(schemaName)
	if schema.Ref != "" {
		fmt.Fprintf(buf, "type %s = %s\n\n", typeName, goName(refName(schema.Ref)))
		return
	}
	if len(schema.OneOf) > 0 && objectVariantCount(schema.OneOf) > 0 {
		fields, required := mergedObjectFields(schema.OneOf)
		writeStruct(buf, spec, typeName, schemaName, fields, required, true)
		return
	}
	if hasType(schema, "object") || len(schema.Properties) > 0 {
		writeStruct(buf, spec, typeName, schemaName, schema.Properties, requiredSet(schema.Required), strings.HasSuffix(schemaName, "Request"))
		return
	}
	if hasType(schema, "string") {
		fmt.Fprintf(buf, "type %s string\n\n", typeName)
		return
	}
	fmt.Fprintf(buf, "type %s any\n\n", typeName)
}

func writeStruct(buf *bytes.Buffer, spec openAPISpec, typeName string, schemaName string, fields map[string]schemaObject, required map[string]bool, requestType bool) {
	fmt.Fprintf(buf, "type %s struct {\n", typeName)
	names := sortedKeys(fields)
	for _, name := range names {
		field := fields[name]
		fieldName := goName(name)
		jsonTag := name
		if requestType && !required[name] {
			jsonTag += ",omitempty"
		}
		fmt.Fprintf(buf, "\t%s %s `json:\"%s\"`\n", fieldName, goType(spec, field, required[name]), jsonTag)
	}
	if len(names) == 0 {
		fmt.Fprintf(buf, "\tAdditionalProperties map[string]any `json:\"-\"`\n")
	}
	buf.WriteString("}\n\n")
}

func generatedClient(ops []operationSpec) string {
	var buf bytes.Buffer
	writeGeneratedHeader(&buf)
	buf.WriteString("package tossapi\n\n")
	buf.WriteString("import (\n")
	buf.WriteString("\t\"context\"\n")
	buf.WriteString("\t\"errors\"\n")
	buf.WriteString("\t\"net/url\"\n")
	buf.WriteString("\t\"strconv\"\n")
	buf.WriteString("\t\"strings\"\n")
	buf.WriteString(")\n\n")
	buf.WriteString("var ErrExecutorRequired = errors.New(\"tossapi executor is required\")\n\n")
	buf.WriteString("type Executor interface {\n")
	buf.WriteString("\tExecuteTossAPI(context.Context, Request, any) error\n")
	buf.WriteString("}\n\n")
	buf.WriteString("type Request struct {\n")
	buf.WriteString("\tOperationID string\n")
	buf.WriteString("\tMethod string\n")
	buf.WriteString("\tPath string\n")
	buf.WriteString("\tQuery url.Values\n")
	buf.WriteString("\tAccountSeq string\n")
	buf.WriteString("\tBody any\n")
	buf.WriteString("\tForm url.Values\n")
	buf.WriteString("\tExpectEnvelope bool\n")
	buf.WriteString("}\n\n")
	for _, op := range ops {
		writeOperationInput(&buf, op)
	}
	for _, op := range ops {
		writeOperationMethod(&buf, op)
	}
	writeClientHelpers(&buf)
	return buf.String()
}

func writeOperationInput(buf *bytes.Buffer, op operationSpec) {
	fmt.Fprintf(buf, "type %sRequest struct {\n", op.GoName)
	for _, param := range op.Parameters {
		fmt.Fprintf(buf, "\t%s %s `json:\"%s,omitempty\"`\n", param.FieldName, param.GoType, param.Name)
	}
	if op.RequestBody != nil && op.RequestBody.GoType != "" {
		fmt.Fprintf(buf, "\tBody %s `json:\"body\"`\n", op.RequestBody.GoType)
	}
	buf.WriteString("}\n\n")
}

func writeOperationMethod(buf *bytes.Buffer, op operationSpec) {
	fmt.Fprintf(buf, "func %s(ctx context.Context, executor Executor, input %sRequest) (%s, error) {\n", op.GoName, op.GoName, op.ResponseGoType)
	fmt.Fprintf(buf, "\tvar result %s\n", op.ResponseGoType)
	buf.WriteString("\tif executor == nil {\n")
	buf.WriteString("\t\treturn result, ErrExecutorRequired\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\trequest := Request{\n")
	fmt.Fprintf(buf, "\t\tOperationID: Operation%s,\n", op.GoName)
	fmt.Fprintf(buf, "\t\tMethod: %q,\n", op.Method)
	if hasPathParams(op) {
		fmt.Fprintf(buf, "\t\tPath: buildPath(Endpoint%s", op.GoName)
		for _, param := range op.Parameters {
			if param.Source == "path" {
				fmt.Fprintf(buf, ", %q, input.%s", param.Name, param.FieldName)
			}
		}
		buf.WriteString("),\n")
	} else {
		fmt.Fprintf(buf, "\t\tPath: Endpoint%s,\n", op.GoName)
	}
	if hasQueryParams(op) {
		fmt.Fprintf(buf, "\t\tQuery: %sQuery(input),\n", unexportedName(op.GoName))
	}
	if accountParam := accountParam(op); accountParam != nil {
		fmt.Fprintf(buf, "\t\tAccountSeq: input.%s,\n", accountParam.FieldName)
	}
	if op.RequestBody != nil {
		switch {
		case op.RequestBody.Form:
			fmt.Fprintf(buf, "\t\tForm: %sForm(input.Body),\n", unexportedName(op.RequestBody.GoType))
		case op.RequestBody.EmptyObject:
			buf.WriteString("\t\tBody: map[string]any{},\n")
		default:
			buf.WriteString("\t\tBody: input.Body,\n")
		}
	}
	fmt.Fprintf(buf, "\t\tExpectEnvelope: %t,\n", op.ExpectEnvelope)
	buf.WriteString("\t}\n")
	buf.WriteString("\terr := executor.ExecuteTossAPI(ctx, request, &result)\n")
	buf.WriteString("\treturn result, err\n")
	buf.WriteString("}\n\n")
	if hasQueryParams(op) {
		writeQueryBuilder(buf, op)
	}
}

func writeQueryBuilder(buf *bytes.Buffer, op operationSpec) {
	fmt.Fprintf(buf, "func %sQuery(input %sRequest) url.Values {\n", unexportedName(op.GoName), op.GoName)
	buf.WriteString("\tquery := url.Values{}\n")
	for _, param := range op.Parameters {
		if param.Source != "query" {
			continue
		}
		switch param.GoType {
		case "int", "int64":
			fmt.Fprintf(buf, "\taddQueryInt(query, %q, int(input.%s))\n", param.Name, param.FieldName)
		case "*bool":
			fmt.Fprintf(buf, "\taddQueryBool(query, %q, input.%s)\n", param.Name, param.FieldName)
		default:
			fmt.Fprintf(buf, "\taddQueryString(query, %q, string(input.%s))\n", param.Name, param.FieldName)
		}
	}
	buf.WriteString("\treturn query\n")
	buf.WriteString("}\n\n")
}

func writeClientHelpers(buf *bytes.Buffer) {
	buf.WriteString("func oAuth2TokenRequestForm(input OAuth2TokenRequest) url.Values {\n")
	buf.WriteString("\tform := url.Values{}\n")
	buf.WriteString("\taddQueryString(form, \"grant_type\", input.GrantType)\n")
	buf.WriteString("\taddQueryString(form, \"client_id\", input.ClientID)\n")
	buf.WriteString("\taddQueryString(form, \"client_secret\", input.ClientSecret)\n")
	buf.WriteString("\treturn form\n")
	buf.WriteString("}\n\n")
	buf.WriteString("func buildPath(template string, pairs ...string) string {\n")
	buf.WriteString("\tout := template\n")
	buf.WriteString("\tfor i := 0; i+1 < len(pairs); i += 2 {\n")
	buf.WriteString("\t\tout = strings.ReplaceAll(out, \"{\"+pairs[i]+\"}\", url.PathEscape(pairs[i+1]))\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn out\n")
	buf.WriteString("}\n\n")
	buf.WriteString("func addQueryString(query url.Values, key string, value string) {\n")
	buf.WriteString("\tif strings.TrimSpace(value) != \"\" {\n")
	buf.WriteString("\t\tquery.Set(key, value)\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")
	buf.WriteString("func addQueryInt(query url.Values, key string, value int) {\n")
	buf.WriteString("\tif value > 0 {\n")
	buf.WriteString("\t\tquery.Set(key, strconv.Itoa(value))\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")
	buf.WriteString("func addQueryBool(query url.Values, key string, value *bool) {\n")
	buf.WriteString("\tif value != nil {\n")
	buf.WriteString("\t\tquery.Set(key, strconv.FormatBool(*value))\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n")
}

func resolveParameter(spec openAPISpec, param parameterObject) (parameterObject, error) {
	if param.Ref == "" {
		return param, nil
	}
	name := refName(param.Ref)
	resolved, ok := spec.Components.Parameters[name]
	if !ok {
		return parameterObject{}, fmt.Errorf("unknown parameter ref %s", param.Ref)
	}
	return resolved, nil
}

func parameterFieldName(param parameterObject) string {
	if strings.EqualFold(param.Name, "X-Tossinvest-Account") {
		return "AccountSeq"
	}
	return goName(param.Name)
}

func parameterGoType(spec openAPISpec, param parameterObject) string {
	if strings.EqualFold(param.Name, "X-Tossinvest-Account") {
		return "string"
	}
	if param.Schema.Ref != "" {
		return goName(refName(param.Schema.Ref))
	}
	if hasType(param.Schema, "boolean") {
		if param.Required {
			return "bool"
		}
		return "*bool"
	}
	if hasType(param.Schema, "integer") {
		if schemaFormat(param.Schema) == "int64" {
			return "int64"
		}
		return "int"
	}
	return "string"
}

func goType(spec openAPISpec, schema schemaObject, required bool) string {
	nullable := schemaNullable(schema)
	if schema.Ref != "" {
		name := goName(refName(schema.Ref))
		if nullable {
			return "*" + name
		}
		return name
	}
	if len(schema.AllOf) == 1 {
		return goType(spec, schema.AllOf[0], required)
	}
	if len(schema.OneOf) > 0 {
		if ref, ok := nullableRef(schema.OneOf); ok {
			return "*" + goName(refName(ref))
		}
		if objectVariantCount(schema.OneOf) > 0 {
			return "map[string]any"
		}
	}
	if len(schema.AnyOf) > 0 {
		if ref, ok := nullableRef(schema.AnyOf); ok {
			return "*" + goName(refName(ref))
		}
	}
	if hasType(schema, "array") {
		if schema.Items == nil {
			return "[]any"
		}
		return "[]" + goType(spec, *schema.Items, true)
	}
	if hasType(schema, "object") || len(schema.Properties) > 0 {
		if len(schema.Properties) == 0 {
			return "map[string]any"
		}
		return "map[string]any"
	}
	if hasType(schema, "boolean") {
		if nullable {
			return "*bool"
		}
		return "bool"
	}
	if hasType(schema, "integer") {
		typeName := "int"
		if schemaFormat(schema) == "int64" {
			typeName = "int64"
		}
		if nullable {
			return "*" + typeName
		}
		return typeName
	}
	if hasType(schema, "number") {
		if nullable {
			return "*json.Number"
		}
		return "json.Number"
	}
	if nullable {
		return "*string"
	}
	if len(schemaTypes(schema)) == 0 && schema.Format == "" && schema.Ref == "" {
		return "any"
	}
	return "string"
}

func hasType(schema schemaObject, want string) bool {
	for _, typ := range schemaTypes(schema) {
		if typ == want {
			return true
		}
	}
	return false
}

func schemaTypes(schema schemaObject) []string {
	switch value := schema.Type.(type) {
	case string:
		return []string{value}
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func schemaNullable(schema schemaObject) bool {
	for _, typ := range schemaTypes(schema) {
		if typ == "null" {
			return true
		}
	}
	return false
}

func schemaFormat(schema schemaObject) string {
	return strings.TrimSpace(schema.Format)
}

func nullableRef(items []schemaObject) (string, bool) {
	ref := ""
	hasNull := false
	for _, item := range items {
		if item.Ref != "" {
			ref = item.Ref
		}
		if hasType(item, "null") {
			hasNull = true
		}
	}
	return ref, ref != "" && hasNull
}

func objectVariantCount(items []schemaObject) int {
	count := 0
	for _, item := range items {
		if hasType(item, "object") || len(item.Properties) > 0 {
			count++
		}
	}
	return count
}

func mergedObjectFields(items []schemaObject) (map[string]schemaObject, map[string]bool) {
	fields := map[string]schemaObject{}
	requiredCounts := map[string]int{}
	objectCount := 0
	for _, item := range items {
		if !(hasType(item, "object") || len(item.Properties) > 0) {
			continue
		}
		objectCount++
		for name, field := range item.Properties {
			fields[name] = field
		}
		for _, name := range item.Required {
			requiredCounts[name]++
		}
	}
	required := map[string]bool{}
	for name, count := range requiredCounts {
		required[name] = count == objectCount
	}
	return fields, required
}

func requiredSet(required []string) map[string]bool {
	out := make(map[string]bool, len(required))
	for _, name := range required {
		out[name] = true
	}
	return out
}

func refName(ref string) string {
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1]
}

func hasQueryParams(op operationSpec) bool {
	for _, param := range op.Parameters {
		if param.Source == "query" {
			return true
		}
	}
	return false
}

func hasPathParams(op operationSpec) bool {
	for _, param := range op.Parameters {
		if param.Source == "path" {
			return true
		}
	}
	return false
}

func accountParam(op operationSpec) *paramSpec {
	for i := range op.Parameters {
		if op.Parameters[i].Source == "header" && op.Parameters[i].FieldName == "AccountSeq" {
			return &op.Parameters[i]
		}
	}
	return nil
}

func isLiveTradingOperation(operationID string) bool {
	switch operationID {
	case "createOrder", "modifyOrder", "cancelOrder":
		return true
	default:
		return false
	}
}

func unexportedName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func goName(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return "Value"
	}
	var b strings.Builder
	for _, word := range words {
		lower := strings.ToLower(word)
		switch lower {
		case "api":
			b.WriteString("API")
		case "id":
			b.WriteString("ID")
		case "url":
			b.WriteString("URL")
		case "uri":
			b.WriteString("URI")
		case "json":
			b.WriteString("JSON")
		case "oauth2":
			b.WriteString("OAuth2")
		case "kr":
			b.WriteString("KR")
		case "us":
			b.WriteString("US")
		case "krx":
			b.WriteString("KRX")
		case "isin":
			b.WriteString("ISIN")
		default:
			runes := []rune(lower)
			if len(runes) == 0 {
				continue
			}
			b.WriteRune(unicode.ToUpper(runes[0]))
			b.WriteString(string(runes[1:]))
		}
	}
	if b.Len() == 0 {
		return "Value"
	}
	out := b.String()
	if unicode.IsDigit([]rune(out)[0]) {
		return "Value" + out
	}
	return out
}

func splitWords(s string) []string {
	s = regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(s, " ")
	var words []string
	for _, raw := range strings.Fields(s) {
		var current []rune
		runes := []rune(raw)
		for i, r := range runes {
			if i > 0 && shouldSplitWord(runes[i-1], r) {
				words = append(words, string(current))
				current = current[:0]
			}
			current = append(current, r)
		}
		if len(current) > 0 {
			words = append(words, string(current))
		}
	}
	return words
}

func shouldSplitWord(prev rune, current rune) bool {
	if unicode.IsUpper(current) && (unicode.IsLower(prev) || unicode.IsDigit(prev)) {
		return true
	}
	if unicode.IsDigit(current) && unicode.IsLetter(prev) {
		return false
	}
	if unicode.IsLetter(current) && unicode.IsDigit(prev) {
		return true
	}
	return false
}

func cleanText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func removeGeneratedFiles(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_gen.go") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func writeGeneratedHeader(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "// Code generated by %s; DO NOT EDIT.\n\n", generatorName)
}

func writeGo(path string, source string) error {
	formatted, err := format.Source([]byte(source))
	if err != nil {
		return fmt.Errorf("format %s: %w\n%s", path, err, source)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
