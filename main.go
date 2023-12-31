package main

import (
	"context"
	"encoding/json"
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"

	"github.com/iancoleman/orderedmap"
	"github.com/podhmo/commentof"
	"github.com/podhmo/commentof/collect"
	"github.com/podhmo/flagstruct"
	"golang.org/x/tools/go/packages"
)

type CLIOptions struct {
	Loose       bool     `flag:"loose" help:"if true, treating as additionalProperties:true"`
	NameTags    []string `flag:"name-tag"`
	OverrideTag string   `flag:"override-tag"`
	RefRoot     string   `flag:"ref-root"`
	Indent      string   `flag:"indent"`

	Query       string `flag:"query" required:"true"`
	Title       string `flag:"schema-title" help:"title attribute of jsonschema"`
	Description string `flag:"schema-description" help:"description attribute of jsonschema"`
	SchemaSpec  string `flag:"schema-spec" help:"$schema attribute of jsonschema"`
}

func main() {
	e := Default()
	flagstruct.Parse(e.Config.CLIOptions, func(b *flagstruct.Builder) {
		b.Name = "genschema"
	})

	query := e.Config.Query
	if strings.HasPrefix(query, ".") {
		b, err := exec.Command("go", "list").Output()
		if err != nil {
			log.Fatalf("go list is failed")
		}
		log.Printf("guess query: %q -> %q", query, strings.TrimSpace(string(b))+query)
		query = strings.TrimSpace(string(b)) + query
	}
	if err := run(e, query); err != nil {
		log.Fatalf("!! %+v", err)
	}
}

func run(e *Extractor, query string) error {
	fset := token.NewFileSet()
	ctx := context.Background() // todo: cancel
	cfg := &packages.Config{
		Fset:    fset,
		Context: ctx,
		Tests:   false,
		Mode:    packages.NeedName | packages.NeedTypes | packages.NeedSyntax,
	}

	parts := strings.Split(query, ".")
	pkgpath := strings.Join(parts[:len(parts)-1], ".")
	name := parts[len(parts)-1]

	pkgs, err := packages.Load(cfg, pkgpath)
	if err != nil {
		return fmt.Errorf("load packages: %w", err)
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
	if len(target.Errors) > 0 {
		for _, e := range target.Errors {
			log.Println(e)
		}
		// return fmt.Errorf("invalid package: %q -- %+v", query, target.Errors[0])
	}

	scope := target.Types.Scope()
	ob := scope.Lookup(name)
	if ob == nil {
		return fmt.Errorf("%q is not found in %s", name, target)
	}

	pos := ob.Pos()
	for _, tree := range target.Syntax {
		if tree.Pos() <= pos && pos <= tree.End() {
			commentInfo, err := commentof.File(fset, tree)
			if err != nil {
				log.Printf("failed to load comments, %s", fset.File(tree.Pos()).Name())
				break
			}
			e.Config.commentInfo = commentInfo
			break
		}
	}

	doc, err := e.Extract(target, ob.Type(), nil, nil)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}

	if e.Config.useCounts[ob.Type()] >= 1 {
		refname := e.Config.ResolveName(e.Config, ob.Type().(*types.Named))
		e.Config.defs[refname] = doc
		doc = orderedmap.New()
		doc.Set("$ref", fmt.Sprintf("#/%s/%s", e.Config.RefRoot, refname))
	}
	if len(e.Config.defs) > 0 {
		doc.Set(e.Config.RefRoot, e.Config.defs)
	}

	root := orderedmap.New()
	root.Set("$schema", e.Config.CLIOptions.SchemaSpec)
	root.Set("title", e.Config.CLIOptions.Title)
	root.Set("description", e.Config.CLIOptions.Description)
	for _, k := range doc.Keys() {
		v, _ := doc.Get(k)
		root.Set(k, v)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", e.Config.Indent)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func Default() *Extractor {
	return &Extractor{
		Config: &Config{
			CLIOptions: &CLIOptions{
				NameTags:    []string{"json", "yaml", "toml"},
				OverrideTag: "jsonschema-override",
				RefRoot:     "$defs",
				SchemaSpec:  "http://json-schema.org/draft-07/schema#",
				Indent:      "\t",
				Description: fmt.Sprintf("Generated by `%s`", strings.Join(os.Args, " ")),
			},
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
			seen:      map[string][]types.Type{},
			defs:      map[string]*orderedmap.OrderedMap{},
			useCounts: map[types.Type]int{},
		},
	}
}

type Config struct {
	*CLIOptions

	ResolveName func(*Config, *types.Named) string

	seen      map[string][]types.Type
	defs      map[string]*orderedmap.OrderedMap
	useCounts map[types.Type]int

	commentInfo *collect.Package
}

type Extractor struct {
	Config *Config
}

func (e *Extractor) Extract(pkg *packages.Package, typ types.Type, hist []types.Type, commentInfo *collect.Object) (*orderedmap.OrderedMap, error) {
	switch typ := typ.(type) {
	case *types.Named:
		seen := e.Config.seen
		name := typ.Obj().Name()

		for _, t := range seen[name] {
			if t == typ {
				ret := orderedmap.New()
				refname := e.Config.ResolveName(e.Config, typ)
				ret.Set("$ref", fmt.Sprintf("#/%s/%s", e.Config.RefRoot, refname))
				e.Config.useCounts[typ]++
				return ret, nil
			}
		}

		seen[name] = append(seen[name], typ)
		description := ""
		if e.Config.commentInfo != nil {
			if v, ok := e.Config.commentInfo.Types[name]; ok {
				commentInfo = v
				if commentInfo.Doc != "" {
					description = strings.TrimSpace(commentInfo.Doc)
				} else if commentInfo.Comment != "" {
					description = strings.TrimSpace(commentInfo.Comment)
				}
			}
		}

		doc, err := e.Extract(pkg, typ.Underlying(), append(hist, typ), commentInfo)
		if err != nil {
			return nil, err
		}

		if hist == nil {
			return doc, nil
		}

		if description != "" {
			doc.Set("description", description)
		}

		refname := e.Config.ResolveName(e.Config, typ)
		e.Config.defs[refname] = doc
		ret := orderedmap.New()
		ret.Set("$ref", fmt.Sprintf("#/%s/%s", e.Config.RefRoot, refname))
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
			if _, isPtr := field.Type().(*types.Pointer); isPtr {
				required = false
			}
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

			fieldDef, err := e.Extract(pkg, field.Type(), append(hist, typ), commentInfo)
			if err != nil {
				log.Printf("%s in %s -- %+v", field, hist[len(hist)-1], err)
				continue
			}
			props.Set(name, fieldDef)

			if commentInfo != nil && commentInfo.Fields != nil {
				if cf, ok := commentInfo.Fields[field.Name()]; ok {
					if cf.Doc != "" {
						fieldDef.Set("description", strings.TrimSpace(cf.Doc))
					} else if cf.Comment != "" {
						fieldDef.Set("description", strings.TrimSpace(cf.Comment))
					}
				}
			}

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
	case *types.Map:
		doc := e.guessType(typ)
		items, err := e.Extract(pkg, typ.Elem(), append(hist, typ), commentInfo)
		if err != nil {
			return nil, fmt.Errorf("unexported type %T: %w", typ, err)
		}
		doc.Set("additionalProperties", items)
		return doc, nil
	case interface { // slices,array
		types.Type
		Elem() types.Type
	}:
		doc := e.guessType(typ)
		items, err := e.Extract(pkg, typ.Elem(), append(hist, typ), commentInfo)
		if err != nil {
			return nil, fmt.Errorf("unexported type %T: %w", typ, err)
		}
		doc.Set("items", items)
		return doc, nil
	default:
		doc := e.guessType(typ)
		if doc == nil {
			return nil, fmt.Errorf("unexpected type: %T, %s", typ, typ)
		}
		return doc, nil
	}
}

func (e *Extractor) guessType(typ types.Type) *orderedmap.OrderedMap {
	switch t := typ.(type) {
	case *types.Named:
		return e.guessType(t.Underlying())
	case *types.Pointer:
		return e.guessType(t.Elem())
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
		return doc
	case *types.Array:
		doc := orderedmap.New()
		doc.Set("type", "array")
		doc.Set("maxItems", t.Len())
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
