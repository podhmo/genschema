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
	"strconv"
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
	query := "github.com/podhmo/genschema/examples/structure.S3"

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

	doc.Set(e.Config.RefRoot, e.Config.defs)
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
			NameTags:    []string{"json", "yaml", "toml"},
			OverrideTag: "jsonschema-override",
			RefRoot:     "$defs",
			ResolveName: func(c *Config, named *types.Named) string {
				name := named.Obj().Name()
				candidates := c.seen[name]
				if len(candidates) == 1 {
					return name
				}
				for i, t := range candidates {
					if t == named {
						return fmt.Sprintf("%s%d", name, i)
					}
				}
				return fmt.Sprintf("%s????", name)
			},
			seen: map[string][]types.Type{},
			defs: map[string]*orderedmap.OrderedMap{},
		},
	}
}

type Config struct {
	Loose       bool
	NameTags    []string
	OverrideTag string
	RefRoot     string

	ResolveName func(*Config, *types.Named) string
	seen        map[string][]types.Type
	defs        map[string]*orderedmap.OrderedMap
}

type Extractor struct {
	Config *Config
}

func (e *Extractor) Extract(pkg *packages.Package, typ types.Type, hist []types.Type) (*orderedmap.OrderedMap, error) {
	switch typ := typ.(type) {
	case *types.Named:
		seen := e.Config.seen
		name := typ.Obj().Name()

		for _, t := range seen[name] {
			if t == typ {
				ret := orderedmap.New()
				id := e.Config.ResolveName(e.Config, typ)
				ret.Set("$ref", fmt.Sprintf("#/%s/%s", e.Config.RefRoot, id))
				return ret, nil
			}
		}
		seen[name] = append(seen[name], typ)
		doc, err := e.Extract(pkg, typ.Underlying(), append(hist, typ))
		if err != nil {
			return nil, err
		}

		if hist == nil {
			return doc, nil
		}

		id := e.Config.ResolveName(e.Config, typ)
		e.Config.defs[id] = doc
		ret := orderedmap.New()
		ret.Set("$ref", fmt.Sprintf("#/%s/%s", e.Config.RefRoot, id))
		return ret, nil
	case *types.Struct:
		doc := e.guessType(typ)

		props := orderedmap.New()
		doc.Set("properties", props)

		requiredList := make([]string, 0, typ.NumFields())
		for i := 0; i < typ.NumFields(); i++ {
			field := typ.Field(i)
			if !field.Exported() {
				continue
			}

			tag := reflect.StructTag(typ.Tag(i))
			name := field.Name()
			required := true
			for _, nametag := range e.Config.NameTags {
				if v, ok := tag.Lookup(nametag); ok {
					v, suffix, _ := strings.Cut(v, ",") // e.g. omitempty with json tag
					if strings.Contains(suffix, "omitempty") {
						required = false
					}
					name = v
					break
				}
			}
			if name == "-" {
				continue
			}

			// TODO: description

			fieldDef, err := e.Extract(pkg, field.Type(), append(hist, typ))
			if err != nil {
				log.Printf("field %s %+v of %s", field, err, typ)
				continue
			}
			props.Set(name, fieldDef)

			if v, ok := tag.Lookup("required"); ok {
				if v, err := strconv.ParseBool(v); err == nil {
					required = v
				}
			}
			if v, ok := tag.Lookup(e.Config.OverrideTag); ok {
				b := []byte(strings.ReplaceAll(strings.ReplaceAll(v, `\`, `\\`), "'", "\""))
				overrides := orderedmap.New()
				if err := json.Unmarshal(b, &overrides); err != nil { // enable cache?
					log.Printf("[WARN]  %s: unmarshal json is failed: %q", e.Config.OverrideTag, err)
				}
				for _, k := range overrides.Keys() {
					v, _ := overrides.Get(k)
					if k == "required" {
						required = v.(bool)
						continue
					}
					fieldDef.Set(k, v)
				}
			}
			if required {
				requiredList = append(requiredList, name)
			}
		}

		if len(requiredList) > 0 {
			doc.Set("required", requiredList)
		}
		// TODO: required
		return doc, nil
	default:
		doc := e.guessType(typ)
		if doc == nil {
			return nil, fmt.Errorf("unexpected type: %T", typ)
		}
		return doc, nil
	}
}

func (e *Extractor) guessType(typ types.Type) *orderedmap.OrderedMap {
	switch t := typ.(type) {
	case *types.Named:
		return e.guessType(t.Underlying())
	case *types.Basic:
		doc := orderedmap.New()
		switch t.Kind() {
		case types.Bool:
			doc.Set("type", "boolean")
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64:
			doc.Set("type", "integer")
		case types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			doc.Set("type", "integer")
			doc.Set("minimum", 0)
		case types.String: // TODO: []byte
			doc.Set("type", "string")
		default:
			doc.Set("type", fmt.Sprintf("go:%s", t.String())) // invalid
		}
		return doc
	case *types.Slice:
		doc := orderedmap.New()
		doc.Set("type", "array")
		doc.Set("items", e.guessType(t.Elem()))
		return doc
	case *types.Array:
		doc := orderedmap.New()
		doc.Set("type", "array")
		doc.Set("maxItems", t.Len())
		doc.Set("items", e.guessType(t.Elem()))
		return doc
	case *types.Map:
		doc := orderedmap.New()
		doc.Set("type", "object")
		doc.Set("additionalProperties", e.guessType(t.Elem()))
		return doc
	case *types.Interface:
		if t.NumMethods() == 0 {
			doc := orderedmap.New()
			doc.Set("type", "object")
			doc.Set("additionalProperties", true)
			doc.Set("description", "any (interface{})")
			return doc
		}
		return nil
	case *types.Struct:
		doc := orderedmap.New()
		doc.Set("type", "object")
		doc.Set("additionalProperties", e.Config.Loose)
		return doc
	default:
		log.Printf("\tunexpected type %T", typ)
		return nil
	}
}
