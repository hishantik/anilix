package hianime

import "github.com/hishantik/anilix/provider"

func init() {
	provider.Register(&provider.Provider{
		ID:   "hianime",
		Name: "Hianime",
		CreateSource: func() (interface{}, error) {
			return NewProvider(), nil
		},
	})
}
