package provider

type Provider struct {
	ID           string
	Name         string
	CreateSource func() (interface{}, error)
}

func (p Provider) String() string {
	return p.Name
}

var providers = []*Provider{}

func All() []*Provider {
	return providers
}

func Register(p *Provider) {
	providers = append(providers, p)
}
