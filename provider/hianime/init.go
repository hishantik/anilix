package hianime

import "github.com/hishantik/anilix/provider"

func init() {
	provider.Register(&provider.Provider{
		ID:           "hianime",
		Name:         "Hianime",
		UsesHeadless: false,
		IsCustom:     false,
		CreateSource: func() (interface{}, error) {
			return NewProvider(), nil
		},
	})
}
