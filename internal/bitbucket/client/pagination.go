package client

import "strconv"

type Page[T any] struct {
	Values        []T  `json:"values"`
	Start         int  `json:"start"`
	Limit         int  `json:"limit"`
	Size          int  `json:"size"`
	IsLastPage    bool `json:"isLastPage"`
	NextPageStart *int `json:"nextPageStart,omitempty"`
}

func (p Page[T]) NextStart() *int {
	return p.NextPageStart
}

func (p Page[T]) NextQuery(query map[string][]string) map[string][]string {
	if p.NextPageStart == nil {
		return nil
	}
	next := make(map[string][]string, len(query)+1)
	for key, values := range query {
		next[key] = append([]string(nil), values...)
	}
	next["start"] = []string{strconv.Itoa(*p.NextPageStart)}
	return next
}
