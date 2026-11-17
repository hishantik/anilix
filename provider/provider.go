package provider

type Provider struct {
	ID           string
	Name         string
	UsesHeadless bool
	IsCustom     bool
	CreateSource func() (interface{}, error)
}

func (p Provider) String() string {
	return p.Name
}

var providers = []*Provider{}

func All() []*Provider {
	return providers
}

func Get(name string) (*Provider, bool) {
	for _, p := range providers {
		if p.Name == name {
			return p, true
		}
	}
	return nil, false
}

func Register(p *Provider) {
	providers = append(providers, p)
}
