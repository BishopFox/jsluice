package jsluice

type set map[string]any

func newSet(items []string) set {
	s := make(set)
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

func (s set) Contains(item string) bool {
	_, exists := s[item]
	return exists
}
