//go:build !embed

package main

import "embed"

var buildFS embed.FS

var indexPage []byte
