GO ?= go

PHONY: run-example
run-example:
	GOBIN=$(shell pwd)/bin $(GO) install -v .
	./bin/genschema --query github.com/podhmo/genschema/_examples/structure.S > _examples/structure/testdata/S.jsonschema.json
	./bin/genschema --query github.com/podhmo/genschema/_examples/structure.S2 > _examples/structure/testdata/S2.jsonschema.json
	./bin/genschema --query github.com/podhmo/genschema/_examples/structure.S3 > _examples/structure/testdata/S3.jsonschema.json
	./bin/genschema --query github.com/podhmo/genschema/_examples/structure.S4 > _examples/structure/testdata/S4.jsonschema.json
	$(GO) get github.com/deepmap/oapi-codegen
	./bin/genschema --schema-title oapi-conf --indent "  " --query github.com/deepmap/oapi-codegen/pkg/codegen.Configuration > _examples/structure/testdata/oapi-codegen.jsonschema.json
	$(GO) get github.com/sqlc-dev/sqlc
	./bin/genschema --schema-title sqlc-conf --indent "  " --query github.com/sqlc-dev/sqlc/internal/config.Config > _examples/structure/testdata/sqlc.jsonschema.json
	$(GO) get github.com/99designs/gqlgen
	./bin/genschema --schema-title gqlgen-conf --indent "  " --query github.com/99designs/gqlgen/codegen/config.Config > _examples/structure/testdata/gqlgen.jsonschema.json
	$(GO) mod tidy