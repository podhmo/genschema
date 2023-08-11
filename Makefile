PHONY: run-example
run-example:
	go run ./ --query github.com/podhmo/genschema/examples/structure.S2 > examples/structure/testdata/S2.jsonschema.json
	go get github.com/deepmap/oapi-codegen
	go run ./ --query github.com/deepmap/oapi-codegen/pkg/codegen.Configuration > examples/structure/testdata/Configuration.jsonschema.json
	go mod tidy