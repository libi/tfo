package search

import "errors"

var (
	// ErrIndexNotOpen 索引未打开
	ErrIndexNotOpen = errors.New("search index is not open")
)
