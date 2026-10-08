package dash

import "github.com/go-sphere/sphere-telegram-layout/internal/pkg/httpsrv"

// DefaultSeedUsername is the dashboard admin username seeded when
// dash.seed_user.username is empty. There is deliberately no default password.
const DefaultSeedUsername = "admin"

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
	Static  string   `json:"static" yaml:"static"`

	httpsrv.Options `yaml:",inline"`
}

// SeedUserConfig is the dashboard admin created on first start, when the
// database has no admin yet. An empty Password makes the seed task generate a
// random one and log it once.
type SeedUserConfig struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

type Config struct {
	AuthJWT    string         `json:"auth_jwt" yaml:"auth_jwt"`
	RefreshJWT string         `json:"refresh_jwt" yaml:"refresh_jwt"`
	HTTP       HTTPConfig     `json:"http" yaml:"http"`
	SeedUser   SeedUserConfig `json:"seed_user" yaml:"seed_user"`
}
