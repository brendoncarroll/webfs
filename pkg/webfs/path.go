package webfs

import "strings"

type Path []string

func (p Path) String() string {
	return "/" + strings.Join(p, "/")
}

func ParsePath(x string) Path {
	y := []string{}
	for _, part := range strings.Split(x, "/") {
		switch part {
		case "", ".":
		default:
			y = append(y, part)
		}
	}
	return Path(y)
}
