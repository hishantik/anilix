package jikan

import "github.com/hishantik/anilix/provider"

func init() {
	provider.Register(&provider.Provider{
		ID:   "jikan",
		Name: "Jikan",
		CreateSource: func() (interface{}, error) {
			return NewJikanProvider(), nil
		},
	})
}
