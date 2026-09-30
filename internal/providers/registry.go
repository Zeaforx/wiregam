package providers

type Provider struct {
	ID          string
	Name        string
	Description string
	IsAvailable bool
}

var Registry = []Provider{
	{
		ID:          "aws",
		Name:        "Amazon Web Services",
		Description: "Deploy WireGuard to AWS EC2",
		IsAvailable: true,
	},
	{
		ID:          "digitalocean",
		Name:        "DigitalOcean",
		Description: "Deploy WireGuard to a DigitalOcean Droplet",
		IsAvailable: false,
	},
}

// Find returns provider metadata by its command-line ID.
func Find(id string) (Provider, bool) {
	for _, provider := range Registry {
		if provider.ID == id {
			return provider, true
		}
	}

	return Provider{}, false
}
