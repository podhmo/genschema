package structure

type S struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// TODO:
// - unexported field
// - json `-`
// - toml,yaml
// - slices,map
// - nested
