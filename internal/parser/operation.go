package parser

import (
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/xseman/openapi-generator/internal/codegen"
)

// GetOperations extracts all operations grouped by tag.
func (p *Parser) GetOperations() (map[string][]*codegen.CodegenOperation, error) {
	if p.Doc == nil || p.Doc.Paths == nil {
		return nil, nil
	}

	operationsByTag := make(map[string][]*codegen.CodegenOperation)

	// Get paths in sorted order
	pathNames := make([]string, 0, p.Doc.Paths.Len())
	for path := range p.Doc.Paths.Map() {
		pathNames = append(pathNames, path)
	}
	sort.Strings(pathNames)

	for _, path := range pathNames {
		pathItem := p.Doc.Paths.Value(path)
		if pathItem == nil {
			continue
		}

		// Process each HTTP method. Use a fixed-order slice rather than a map:
		// Go map iteration is randomised, which would otherwise shuffle the order
		// of generated methods within a path (and thus within each API class).
		methods := []struct {
			name string
			op   *openapi3.Operation
		}{
			{"GET", pathItem.Get},
			{"POST", pathItem.Post},
			{"PUT", pathItem.Put},
			{"DELETE", pathItem.Delete},
			{"PATCH", pathItem.Patch},
			{"OPTIONS", pathItem.Options},
			{"HEAD", pathItem.Head},
		}

		for _, m := range methods {
			method, op := m.name, m.op
			if op == nil {
				continue
			}

			// Match upstream: DefaultGenerator's path-processing loop iterates every
			// tag on the operation (defaulting to "default" when there are none) and
			// builds+adds a CodegenOperation once per tag (DefaultGenerator.java
			// ~1593-1602: `for (Tag tag : tags) { ... fromOperation(...);
			// addOperationToGroup(...) }`), so an operation tagged with several tags
			// is emitted into every one of their API files, not just the first.
			// operationToCodegen is called fresh per tag (rather than building once
			// and sharing the pointer across groups) both to mirror that re-invocation
			// and because each copy needs its own BaseName.
			tags := op.Tags
			if len(tags) == 0 {
				tags = []string{"default"}
			}
			for _, tag := range tags {
				operation := p.operationToCodegen(path, method, op, pathItem.Parameters)
				operation.BaseName = tag
				operationsByTag[tag] = append(operationsByTag[tag], operation)
			}
		}
	}

	// Match upstream: DefaultGenerator.generateApis sorts each tag's operation
	// list by operationId (ObjectUtils.compare, i.e. plain ascending string
	// comparison) unless sorting is explicitly skipped. Sorting here — rather
	// than relying on path/method iteration order — is what the Java baseline
	// actually does, and Nickname is the lowerCamelCase identifier Java's
	// toOperationId produces and sorts on (it's also the name templates emit
	// as the method name).
	//
	// This must be a *stable* sort: Java's List.sort (TimSort) is stable, and
	// with duplicate operationIds — which Java tolerates and resolves into
	// distinct nicknames elsewhere — an unstable sort can permute same-key
	// operations into an order that diverges from Java's for reasons unrelated
	// to the sort key itself.
	//
	// NOTE: this sorts on Nickname as currently assigned to each operation,
	// i.e. *before* any dedup/collision suffixing a caller applies afterwards
	// (see cmd/openapi-generator/main.go, which resolves duplicate operationIds
	// within a tag after calling GetOperations). Java resolves the equivalent
	// collision (addOperationToGroup, DefaultCodegen.java ~5790-5819) before it
	// sorts, so two operations that collide pre-dedup here can still end up
	// ordered differently than Java once the caller's suffixing changes their
	// relative Nickname order. Fixing that requires moving collision
	// resolution to run before this sort, which is out of this file's scope —
	// see the parity report for what needs to move and where.
	for tag, ops := range operationsByTag {
		sort.SliceStable(ops, func(i, j int) bool {
			return ops[i].Nickname < ops[j].Nickname
		})
		operationsByTag[tag] = ops
	}

	return operationsByTag, nil
}

// operationToCodegen converts an OpenAPI operation to a CodegenOperation.
func (p *Parser) operationToCodegen(path, method string, op *openapi3.Operation, pathParams openapi3.Parameters) *codegen.CodegenOperation {
	co := &codegen.CodegenOperation{
		Path:                path,
		HttpMethod:          method,
		OperationId:         op.OperationID,
		OperationIdOriginal: op.OperationID,
		Summary:             escapeUnsafeChars(op.Summary),
		Notes:               escapeUnsafeChars(op.Description),
		UnescapedNotes:      escapeUnsafeChars(op.Description),
		IsDeprecated:        op.Deprecated,
		VendorExtensions:    convertExtensions(op.Extensions),
	}

	// Generate operation ID if not provided
	if co.OperationId == "" {
		co.OperationId = strings.ToLower(method) + sanitizeTag(path)
	}

	// Set operation ID variants.
	// OperationIdCamelCase is UpperCamelCase (PascalCase) and is used for type-level
	// identifiers such as the request interface and inline parameter enums. Nickname is
	// lowerCamelCase and is used for the generated method names.
	co.OperationIdCamelCase = toPascalCase(co.OperationId)
	co.OperationIdLowerCase = strings.ToLower(co.OperationId)
	co.OperationIdSnakeCase = toSnakeCase(co.OperationId)
	co.Nickname = toCamelCase(co.OperationId)

	// Set tag/baseName
	if len(op.Tags) > 0 {
		co.BaseName = op.Tags[0]
	} else {
		co.BaseName = "default"
	}

	// Process path parameters from path item
	for _, paramRef := range pathParams {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := p.parameterToCodegen(paramRef.Value)
		co.AllParams = append(co.AllParams, param)
		co.PathParams = append(co.PathParams, param)
	}

	// Process operation parameters
	for _, paramRef := range op.Parameters {
		if paramRef == nil || paramRef.Value == nil {
			continue
		}
		param := p.parameterToCodegen(paramRef.Value)
		co.AllParams = append(co.AllParams, param)

		switch paramRef.Value.In {
		case "path":
			co.PathParams = append(co.PathParams, param)
		case "query":
			co.QueryParams = append(co.QueryParams, param)
		case "header":
			co.HeaderParams = append(co.HeaderParams, param)
		case "cookie":
			co.CookieParams = append(co.CookieParams, param)
		}

		if param.Required {
			co.RequiredParams = append(co.RequiredParams, param)
		} else {
			co.OptionalParams = append(co.OptionalParams, param)
			co.HasOptionalParams = true
		}
	}

	// Process request body
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		body := op.RequestBody.Value

		// Iterate content types in a deterministic order. Go map iteration is randomised,
		// which would otherwise make the chosen schema/content-type (and therefore the
		// generated output) unstable across runs.
		contentTypes := make([]string, 0, len(body.Content))
		for ct := range body.Content {
			contentTypes = append(contentTypes, ct)
		}
		sort.Strings(contentTypes)

		for _, contentType := range contentTypes {
			mediaType := body.Content[contentType]
			if mediaType.Schema == nil || mediaType.Schema.Value == nil {
				continue
			}
			schema := mediaType.Schema.Value

			// multipart/form-data and form-urlencoded bodies expand into one form
			// parameter per schema property (e.g. `file`), matching upstream
			// openapi-generator, rather than a single opaque `body` parameter.
			isForm := strings.HasPrefix(contentType, "multipart/") ||
				contentType == "application/x-www-form-urlencoded"
			if isForm && isObjectSchema(schema) && len(schema.Properties) > 0 {
				p.addFormParams(co, schema, contentType)
				if strings.HasPrefix(contentType, "multipart/") {
					co.IsMultipart = true
				}
				break
			}

			bodyParam := &codegen.CodegenParameter{
				IsBodyParam: true,
				Required:    body.Required,
				IsNullable:  schema.Nullable,
				Description: escapeUnsafeChars(body.Description),
				ContentType: contentType,
			}

			if mediaType.Schema.Ref != "" {
				refName := extractRefName(mediaType.Schema.Ref)
				modelName := p.toModelName(refName)
				// Use "any" if model name is empty
				if modelName == "" {
					modelName = "any"
				}
				bodyParam.DataType = modelName
				bodyParam.BaseType = modelName
				bodyParam.IsModel = true
				// Name a $ref body after the referenced schema (camelCased).
				bodyParam.BaseName = refName
				bodyParam.ParamName = p.toVarName(refName)
			} else {
				// Derive the full property shape so inline bodies (arrays, primitives,
				// maps) carry their container/item information instead of collapsing to a
				// bare type. Without this an array body renders as `Array` and an
				// undefined `ArrayToJSON` helper.
				prop := p.schemaToProperty("body", schema, body.Required)
				bodyParam.DataType = prop.DataType
				bodyParam.BaseType = prop.BaseType
				bodyParam.IsArray = prop.IsArray
				bodyParam.IsMap = prop.IsMap
				bodyParam.IsContainer = prop.IsContainer
				bodyParam.IsPrimitiveType = prop.IsPrimitiveType
				bodyParam.IsModel = prop.IsModel
				bodyParam.IsString = prop.IsString
				bodyParam.IsNumber = prop.IsNumber
				bodyParam.IsInteger = prop.IsInteger
				bodyParam.IsBoolean = prop.IsBoolean
				bodyParam.Items = prop.Items
				bodyParam.ComposedModels = prop.ComposedModels

				// Union/intersection bodies have no single ToJSON helper; treat
				// them as primitive so the template passes the value through.
				// applyCompositeType marks these free-form without the primitive
				// flag, which also covers unions collapsed to a single member.
				if isCompositeType(prop.DataType) || (prop.IsFreeFormObject && !prop.IsPrimitiveType) {
					bodyParam.IsPrimitiveType = true
					bodyParam.IsModel = false
					bodyParam.IsContainer = false
				}

				// Fall back to the legacy declaration for composed schemas (allOf/oneOf)
				// that schemaToProperty cannot resolve to a concrete model.
				if (bodyParam.DataType == "" || bodyParam.DataType == "any") && len(schema.AllOf) > 0 {
					bodyParam.DataType = p.getTypeDeclaration(schema)
					bodyParam.BaseType = bodyParam.DataType
					// Composite declarations ("A & B") have no ToJSON helper and
					// must be passed through like primitives.
					composite := isCompositeType(bodyParam.DataType)
					bodyParam.IsModel = !isPrimitiveType(bodyParam.DataType) && !composite
					bodyParam.IsPrimitiveType = isPrimitiveType(bodyParam.DataType) || composite
				}

				// Name an inline body following upstream: an array of a model takes the
				// innermost item model name; an array of primitives or a map uses
				// "requestBody"; everything else (primitives, free-form objects) uses "body".
				baseName := "body"
				switch {
				case prop.IsArray:
					inner := prop.Items
					for inner != nil && inner.Items != nil {
						inner = inner.Items
					}
					if inner != nil && inner.IsModel && !isPrimitiveType(inner.DataType) {
						baseName = inner.DataType
					} else {
						baseName = "request_body"
					}
				case prop.IsMap:
					baseName = "request_body"
				}
				bodyParam.BaseName = baseName
				bodyParam.ParamName = p.toVarName(baseName)
			}

			// Use "any" if type declaration is empty
			if bodyParam.DataType == "" {
				bodyParam.DataType = "any"
				bodyParam.BaseType = "any"
			}

			co.BodyParam = bodyParam
			co.BodyParams = append(co.BodyParams, bodyParam)
			co.AllParams = append(co.AllParams, bodyParam)

			// Add to required/optional params
			if bodyParam.Required {
				co.RequiredParams = append(co.RequiredParams, bodyParam)
			} else {
				co.OptionalParams = append(co.OptionalParams, bodyParam)
				co.HasOptionalParams = true
			}

			break // Use first (sorted) content type
		}
	}

	// Process responses
	if op.Responses != nil {
		// Iterate status codes in sorted order. Go map iteration is randomised,
		// which would otherwise shuffle the generated response list and make the
		// chosen 2xx return type unstable across runs.
		respMap := op.Responses.Map()
		codes := make([]string, 0, len(respMap))
		for code := range respMap {
			codes = append(codes, code)
		}
		sort.Strings(codes)

		for _, code := range codes {
			respRef := respMap[code]
			if respRef == nil || respRef.Value == nil {
				continue
			}

			resp := p.responseToCodegen(code, respRef.Value)
			co.Responses = append(co.Responses, resp)

			// Set return type from the first (lowest) 2xx response that carries
			// a body, e.g. prefer 200 over 201 when both are present.
			if co.ReturnType == "" && strings.HasPrefix(code, "2") && resp.DataType != "" {
				co.ReturnType = resp.DataType
				co.ReturnBaseType = resp.BaseType
				co.ReturnComposedModels = resp.ComposedModels
				co.ReturnSimpleType = resp.SimpleType
				co.ReturnTypeIsPrimitive = resp.PrimitiveType
				if resp.IsArray {
					co.IsArray = true
					co.ReturnContainer = "array"
				}
				if resp.IsMap {
					co.IsMap = true
					co.ReturnContainer = "map"
				}
				if resp.IsBinary || resp.IsFile {
					co.IsResponseBinary = true
					co.IsResponseFile = resp.IsFile
				}
			}
		}
	}

	// Set content types in sorted order for stable output.
	if op.RequestBody != nil && op.RequestBody.Value != nil {
		consumes := make([]string, 0, len(op.RequestBody.Value.Content))
		for ct := range op.RequestBody.Value.Content {
			consumes = append(consumes, ct)
		}
		sort.Strings(consumes)
		for _, ct := range consumes {
			co.Consumes = append(co.Consumes, map[string]string{"mediaType": ct})
		}
		co.HasConsumes = len(co.Consumes) > 0
	}

	// Process security. Each requirement is a map of scheme name -> scopes;
	// iterate names in sorted order so AuthMethods is stable across runs.
	if op.Security != nil {
		for _, secReq := range *op.Security {
			names := make([]string, 0, len(secReq))
			for name := range secReq {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				scopes := secReq[name]
				sec := &codegen.CodegenSecurity{
					Name:   name,
					Scopes: make([]map[string]any, len(scopes)),
				}
				for i, scope := range scopes {
					sec.Scopes[i] = map[string]any{"scope": scope}
				}
				co.AuthMethods = append(co.AuthMethods, sec)
			}
		}
		co.HasAuthMethods = len(co.AuthMethods) > 0
	}

	// Deduplicate parameters by name (path params can override operation params)
	co.AllParams = deduplicateParams(co.AllParams)
	co.PathParams = deduplicateParams(co.PathParams)
	co.QueryParams = deduplicateParams(co.QueryParams)
	co.HeaderParams = deduplicateParams(co.HeaderParams)

	// Rebuild required and optional params from deduplicated allParams
	co.RequiredParams = nil
	co.OptionalParams = nil
	co.HasOptionalParams = false
	for _, param := range co.AllParams {
		if param.Required {
			co.RequiredParams = append(co.RequiredParams, param)
		} else {
			co.OptionalParams = append(co.OptionalParams, param)
			co.HasOptionalParams = true
		}
	}

	// Collect imports
	co.Imports = p.collectOperationImports(co)

	return co
}

// addFormParams expands a multipart/form-urlencoded request-body schema into one
// CodegenParameter per property, appending them to the operation's FormParams and
// AllParams. This mirrors upstream openapi-generator, which surfaces form fields (e.g. an
// uploaded `file`) as individual parameters rather than a single opaque body.
func (p *Parser) addFormParams(co *codegen.CodegenOperation, schema *openapi3.Schema, contentType string) {
	requiredSet := make(map[string]bool)
	for _, r := range schema.Required {
		requiredSet[r] = true
	}

	for _, name := range p.orderedPropertyNames(schema) {
		propRef := schema.Properties[name]
		if propRef == nil || propRef.Value == nil {
			continue
		}
		required := requiredSet[name]
		prop := p.schemaRefToProperty(name, propRef, required)

		fp := &codegen.CodegenParameter{
			BaseName:         name,
			ParamName:        prop.Name,
			IsFormParam:      true,
			Required:         required,
			Description:      prop.Description,
			ContentType:      contentType,
			DataType:         prop.DataType,
			DatatypeWithEnum: prop.DatatypeWithEnum,
			BaseType:         prop.BaseType,
			IsArray:          prop.IsArray,
			IsMap:            prop.IsMap,
			IsContainer:      prop.IsContainer,
			IsPrimitiveType:  prop.IsPrimitiveType,
			IsModel:          prop.IsModel,
			IsString:         prop.IsString,
			IsNumber:         prop.IsNumber,
			IsInteger:        prop.IsInteger,
			IsBoolean:        prop.IsBoolean,
			IsBinary:         prop.IsBinary,
			IsFile:           prop.IsFile,
			IsDate:           prop.IsDate,
			IsDateType:       prop.IsDate,
			IsDateTime:       prop.IsDateTime,
			IsDateTimeType:   prop.IsDateTime,
			IsEnum:           prop.IsEnum,
			Items:            prop.Items,
			AllowableValues:  prop.AllowableValues,
			EnumName:         prop.EnumName,
			UniqueItems:      prop.UniqueItems,
		}
		if fp.IsArray {
			fp.CollectionFormat = "csv"
			// An array of files must be sent as FormData with one append per
			// element; joining Blobs into a csv string produces a broken body.
			if prop.Items != nil && (prop.Items.IsFile || prop.Items.IsBinary) {
				fp.IsFile = true
				fp.IsCollectionFormatMulti = true
				fp.CollectionFormat = "multi"
			}
		}

		co.FormParams = append(co.FormParams, fp)
		co.AllParams = append(co.AllParams, fp)
		if required {
			co.RequiredParams = append(co.RequiredParams, fp)
		} else {
			co.OptionalParams = append(co.OptionalParams, fp)
			co.HasOptionalParams = true
		}
	}
}

// deduplicateParams removes duplicate parameters by name, keeping the last occurrence.
// This is useful when path-level and operation-level parameters have the same name.
func deduplicateParams(params []*codegen.CodegenParameter) []*codegen.CodegenParameter {
	if len(params) == 0 {
		return params
	}

	seen := make(map[string]int)
	var result []*codegen.CodegenParameter

	// Iterate through parameters and track positions
	for i, param := range params {
		if param == nil {
			continue
		}
		key := param.ParamName
		if idx, exists := seen[key]; exists {
			// Replace previous occurrence
			result[idx] = param
		} else {
			// First occurrence
			seen[key] = len(result)
			result = append(result, param)
		}
		_ = i // unused
	}

	return result
}
