PHONY: run-example
run-example:
	GOBIN=$(shell pwd)/bin go install -v .
	./bin/genschema --query github.com/podhmo/genschema/examples/structure.S > examples/structure/testdata/S.jsonschema.json
	./bin/genschema --query github.com/podhmo/genschema/examples/structure.S2 > examples/structure/testdata/S2.jsonschema.json
	./bin/genschema --query github.com/podhmo/genschema/examples/structure.S3 > examples/structure/testdata/S3.jsonschema.json
	go get github.com/deepmap/oapi-codegen
	./bin/genschema --query github.com/deepmap/oapi-codegen/pkg/codegen.Configuration > examples/structure/testdata/Configuration.jsonschema.json
	go mod tidy