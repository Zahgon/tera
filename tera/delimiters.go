package tera

// Delimiters configures the two-byte template delimiters.
type Delimiters struct {
	// BlockStart opens a statement tag.
	BlockStart string
	// BlockEnd closes a statement tag.
	BlockEnd string
	// VariableStart opens an output expression.
	VariableStart string
	// VariableEnd closes an output expression.
	VariableEnd string
	// CommentStart opens a comment.
	CommentStart string
	// CommentEnd closes a comment.
	CommentEnd string
}

// DefaultDelimiters returns the standard Jinja-style markers.
func DefaultDelimiters() Delimiters { return Delimiters{"{%", "%}", "{{", "}}", "{#", "#}"} }
func (d Delimiters) validate() error {
	names := []string{"block_start", "block_end", "variable_start", "variable_end", "comment_start", "comment_end"}
	values := []string{d.BlockStart, d.BlockEnd, d.VariableStart, d.VariableEnd, d.CommentStart, d.CommentEnd}
	for i, s := range values {
		if len(s) != 2 {
			return ErrorMessage("`" + names[i] + "` delimiter must be 2 bytes long")
		}
	}
	for _, p := range [][2]int{{0, 2}, {0, 4}, {2, 4}} {
		if values[p[0]] == values[p[1]] {
			return ErrorMessage("`" + names[p[0]] + "` and `" + names[p[1]] + "` cannot have the same value")
		}
	}
	return nil
}
