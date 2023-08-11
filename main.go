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
	query := "github.com/podhmo/genschema/examples/structure.S"

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

	doc, err := extract(target, ob.Type(), nil)
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

func extract(pkg *packages.Package, typ types.Type, named *types.Named) (*orderedmap.OrderedMap, error) {
	switch typ := typ.(type) {
	case *types.Named:
		return extract(pkg, typ.Underlying(), typ)
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
			if v, ok := tag.Lookup("json"); ok {
				name = v
			}
			// TODO: guess type
			fieldDef := orderedmap.New()
			fieldDef.Set("type", guessType(field.Type()))
			props.Set(name, fieldDef)
		}
		return doc, nil
	default:
		return nil, fmt.Errorf("unexpected type %s", typ)
		// never
	}
}
func guessType(typ types.Type) string {
	return typ.Underlying().String() // TODO: integer
}
