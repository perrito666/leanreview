// Package examples holds tiny files used by leanreview's living example
// pull requests — review material, not shipped code.
package examples

import "fmt"

// Greet builds the review greeting.
func Greet(name string) string {
	result := fmt.Sprintf("hello %s", name)
	return result
}
