PHONY: run-example
run-example:
	go run ./ --query github.com/podhmo/genschema/examples/structure.S2 > examples/structure/testdata/S2.jsonschema.json
