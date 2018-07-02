package mulberry

// Option is a functional option type used in mulberry constructors
type Option func(*View)

// Dimensions sets the width and height of a view
func Dimensions(w, h int) Option {
	return func(v *View) {
		v.width = w
		v.height = h
	}
}
