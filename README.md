# genschema
generate jsonschema from go's struct via go/types

## additional tags

- required
- jsonschema-override

## sample data

https://github.com/deepmap/oapi-codegen

```console
$ genschema -schema-title oapi-conf --query github.com/deepmap/oapi-codegen/pkg/codegen.Configuration > _examples/structure/testdata/oapi-codegen.jsonschema.json
```

- jsonschema https://github.com/podhmo/genschema/blob/main/_examples/structure/testdata/oapi-codegen.jsonschema.json
- conf example https://github.com/podhmo/genschema/blob/main/_examples/structure/testdata/oapi-codegen.yaml