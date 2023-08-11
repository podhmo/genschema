package structure

type S struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type PositiveInt int64
type PInt PositiveInt

type S2 struct {
	Name string `json:"name"` // name of object

	// age of object
	Age PInt `json:"age"`

	Nickname string `json:"nickname,omitempty"`

	Friends []string       `json:"friends"`
	Items   map[string]int `json:"items"`

	Ignored string `json:"-"`
}

// TODO:
// - unexported field
// - json `-`
// - toml,yaml
// - slices,map
// - nested
