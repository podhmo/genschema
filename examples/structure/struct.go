package structure

import "fmt"

type S struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type PositiveInt int64
type PInt PositiveInt

type S2 struct {
	Name string `json:"name"` // name of object

	// age of object
	Age PInt `json:"age" required:"false"`

	Nickname string `json:"nickname,omitempty"`

	Friends []string       `json:"friends"`
	Items   map[string]int `json:"items"`

	Ignored string `json:"-"`

	Greeting fmt.Stringer
	Any      any `jsonschema-override:"{'required': false, 'deprecated': true}"`
}

type S3 struct {
	Named   Sub2 `json:"named,omitempty"`
	Unnamed struct {
		Name Name `json:"name"`
	} `json:"unnamed,omitempty"`
}

type Sub2 struct {
	Name Name `json:"name"`
}

type Name string

// TODO:
// - unexported field
// - json `-`
// - toml,yaml
// - slices,map
// - nested
