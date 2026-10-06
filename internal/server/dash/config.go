package dash

// DefaultSeedUsername and DefaultSeedPassword fill dash.seed_user when the
// config file leaves it empty. They only matter on a database without any
// admin, and the password must be changed before the service is exposed.
const (
	DefaultSeedUsername = "admin"
	DefaultSeedPassword = "aA1234567"
)

type HTTPConfig struct {
	Address string   `json:"address" yaml:"address"`
	Cors    []string `json:"cors" yaml:"cors"`
	Static  string   `json:"static" yaml:"static"`
}

// SeedUserConfig is the dashboard admin created on first start, when the
// database has no admin yet.
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
