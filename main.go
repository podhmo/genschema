package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"reflect"
	"strings"

	"github.com/iancoleman/orderedmap"
	"golang.org/x/tools/go/packages"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("!! %+v", err)
	}
}

func run() error {
	fset := token.NewFileSet()
	ctx := context.Background() // todo: cancel
	cfg := &packages.Config{
		Fset:    fset,
		Context: ctx,
		Tests:   false,
		Mode:    packages.NeedName | packages.NeedTypes,
	}

	// TODO: <package path>.<symbol>
	query := "github.com/podhmo/genschema/examples/structure.S2"

	parts := strings.Split(query, ".")
	pkgpath := strings.Join(parts[:len(parts)-1], ".")
	name := parts[len(parts)-1]

	pkgs, err := packages.Load(cfg, pkgpath)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("invalid package: %q", query)
	}

	var target *packages.Package
	for _, pkg := range pkgs {
		if pkg.ID != pkgpath {
			continue
		}
		target = pkg
	}
	if target == nil {
		return fmt.Errorf("%q is not found", query)
	}

	scope := target.Types.Scope()
	ob := scope.Lookup(name)
	if ob == nil {
		return fmt.Errorf("%q is not found in %s", name, target)
	}

	e := Default()
	doc, err := e.Extract(target, ob.Type(), nil)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "\t")
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func Default() *Extractor {
	return &Extractor{
		Config: &Config{
			NameTags: []string{"json", "yaml", "toml"},
		},
	}
}

type Config struct {
	NameTags []string
}

type Extractor struct {
	Config *Config
}

func (e *Extractor) Extract(pkg *packages.Package, typ types.Type, named *types.Named) (*orderedmap.OrderedMap, error) {
	switch typ := typ.(type) {
	case *types.Named:
		return e.Extract(pkg, typ.Underlying(), typ)
	case *types.Struct:
		doc := orderedmap.New()
		doc.Set("type", "object")
		doc.Set("additionalProperties", false)

		props := orderedmap.New()
		doc.Set("properties", props)

		for i := 0; i < typ.NumFields(); i++ {
			field := typ.Field(i)
			if !token.IsExported(field.Name()) {
				continue
			}

			tag := reflect.StructTag(typ.Tag(i))
			name := field.Name()
			for _, nametag := range e.Config.NameTags {
				if v, ok := tag.Lookup(nametag); ok {
					name, _, _ = strings.Cut(v, ",") // e.g. omitempty with json tag
					break
				}
			}
			if name == "-" {
				continue
			}

			// TODO: guess type
			// TODO: description
			fieldDef := orderedmap.New()
			fieldDef.Set("type", guessType(field.Type()))
			props.Set(name, fieldDef)
		}
		// TODO: required
		return doc, nil
	default:
		return nil, fmt.Errorf("unexpected type %s", typ)
		// never
	}
}
func guessType(typ types.Type) string {
	switch inner := typ.Underlying().String(); inner {
	case "int", "int64":
		return "integer"
	case "bool":
		return "boolean"
	case "float", "float64":
		return "number"
	case "string", "[]byte": // xxx
		return "string"
	default:
		return inner
	}

}
