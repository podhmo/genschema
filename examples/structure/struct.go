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

	Ignored string `json:"-"`
}

// TODO:
// - unexported field
// - json `-`
// - toml,yaml
// - slices,map
// - nested
